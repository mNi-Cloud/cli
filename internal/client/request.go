package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mNi-Cloud/cli/internal/api"
)

// messageLimit keeps an unexpected body from filling the terminal.
const messageLimit = 200

func get[T any](ctx context.Context, httpClient *http.Client, url string) (T, error) {
	var zero T

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return zero, err
	}
	return do[T](ctx, httpClient, req)
}

// post calls an endpoint that takes no body, such as a subresource that stands
// for an operation.
func post[T any](ctx context.Context, httpClient *http.Client, url string) (T, error) {
	var zero T

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return zero, err
	}
	return do[T](ctx, httpClient, req)
}

func remove[T any](ctx context.Context, httpClient *http.Client, url string) (T, error) {
	var zero T

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return zero, err
	}
	return do[T](ctx, httpClient, req)
}

func send[T any](ctx context.Context, httpClient *http.Client, method, url, contentType string, body any) (T, error) {
	var zero T

	encoded, err := json.Marshal(body)
	if err != nil {
		return zero, fmt.Errorf("cannot encode the request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(encoded))
	if err != nil {
		return zero, err
	}
	req.Header.Set("Content-Type", contentType)

	return do[T](ctx, httpClient, req)
}

func do[T any](ctx context.Context, httpClient *http.Client, req *http.Request) (T, error) {
	var zero T
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return zero, fmt.Errorf("cannot read the answer of %s: %w", req.URL, err)
	}

	var envelope api.Response[T]
	decodeErr := json.Unmarshal(raw, &envelope)

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return zero, &api.Error{
			StatusCode: resp.StatusCode,
			Message:    failureMessage(envelope, decodeErr, raw),
			Challenge:  resp.Header.Get("WWW-Authenticate"),
		}
	}
	if decodeErr != nil {
		return zero, fmt.Errorf("cannot read the answer of %s: %w", req.URL, decodeErr)
	}
	if !envelope.Success {
		return zero, &api.Error{StatusCode: resp.StatusCode, Message: envelope.Message}
	}

	return envelope.Data, nil
}

func failureMessage[T any](envelope api.Response[T], decodeErr error, raw []byte) string {
	if decodeErr == nil {
		return envelope.Message
	}

	body := strings.TrimSpace(string(raw))
	if len(body) > messageLimit {
		return body[:messageLimit] + "..."
	}
	return body
}
