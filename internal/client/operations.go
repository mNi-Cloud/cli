package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/mNi-Cloud/cli/internal/api"
)

type GuestExecRequest struct {
	Argv             []string `json:"argv"`
	StdinBase64      string   `json:"stdinBase64,omitempty"`
	WorkingDirectory string   `json:"workingDirectory,omitempty"`
}

type GuestExecResponse struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

type CloudShellSession struct {
	ID        string `json:"id"`
	Subnet    string `json:"subnet"`
	Phase     string `json:"phase"`
	CreatedAt string `json:"createdAt"`
}

func (c *Client) GuestExec(ctx context.Context, tenant, name string, request GuestExecRequest) (GuestExecResponse, error) {
	var response GuestExecResponse
	raw, _, err := c.raw(ctx, http.MethodPost, c.tenantObjectPath("vm", "v1alpha1", tenant, "virtualmachines", name)+"/guest-exec", nil, request)
	if err != nil {
		return response, err
	}
	err = json.Unmarshal(raw, &response)
	return response, err
}

func (c *Client) Kubeconfig(ctx context.Context, tenant, name string) ([]byte, http.Header, error) {
	return c.raw(ctx, http.MethodPost, c.tenantObjectPath("k8s", "v1alpha1", tenant, "clusters", name)+"/kubeconfig", nil, nil)
}

func (c *Client) ClusterResources(ctx context.Context, tenant, cluster string, segments []string, query url.Values) ([]byte, error) {
	path := c.tenantObjectPath("k8s", "v1alpha1", tenant, "clusters", cluster) + "/resources"
	for _, segment := range segments {
		if segment == "" {
			return nil, errors.New("no Kubernetes resource segment to address")
		}
		path += "/" + url.PathEscape(segment)
	}
	raw, _, err := c.raw(ctx, http.MethodGet, path, query, nil)
	return raw, err
}

func (c *Client) CloudShellSessions(ctx context.Context, tenant string) ([]CloudShellSession, error) {
	var response struct {
		Sessions []CloudShellSession `json:"sessions"`
	}
	raw, _, err := c.raw(ctx, http.MethodGet, c.cloudShellSessionsPath(tenant), nil, nil)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(raw, &response)
	return response.Sessions, err
}

func (c *Client) CreateCloudShellSession(ctx context.Context, tenant, subnet string) (CloudShellSession, error) {
	var session CloudShellSession
	if subnet == "" {
		return session, errors.New("no subnet for the CloudShell session")
	}
	raw, _, err := c.raw(ctx, http.MethodPost, c.cloudShellSessionsPath(tenant), nil, struct {
		Subnet string `json:"subnet"`
	}{Subnet: subnet})
	if err != nil {
		return session, err
	}
	err = json.Unmarshal(raw, &session)
	return session, err
}

func (c *Client) DeleteCloudShellSession(ctx context.Context, tenant, session string) error {
	if session == "" {
		return errors.New("no CloudShell session to delete")
	}
	_, _, err := c.raw(ctx, http.MethodDelete, c.cloudShellSessionsPath(tenant)+"/"+url.PathEscape(session), nil, nil)
	return err
}

func (c *Client) ConnectPath(ctx context.Context, path string) (Stream, error) {
	target, err := websocketURL(c.baseURL + path)
	if err != nil {
		return nil, err
	}
	token, err := c.tokens.Token(ctx)
	if err != nil {
		return nil, err
	}
	header := http.Header{"Authorization": {"Bearer " + token}}
	dialer := *c.websocket
	dialer.Subprotocols = []string{binarySubprotocol}
	conn, response, err := dialer.DialContext(ctx, target, header)
	if err != nil {
		return nil, handshakeError(err, response)
	}
	return newStream(conn), nil
}

func (c *Client) VMShell(ctx context.Context, tenant, name string) (Stream, error) {
	return c.ConnectPath(ctx, c.tenantObjectPath("vm", "v1alpha1", tenant, "virtualmachines", name)+"/shell")
}

func (c *Client) CloudShell(ctx context.Context, tenant, session string) (Stream, error) {
	if session == "" {
		return nil, errors.New("no CloudShell session to open")
	}
	return c.ConnectPath(ctx, "/cs/v1alpha1/tenants/"+url.PathEscape(tenant)+"/shell/"+url.PathEscape(session))
}

func (c *Client) raw(ctx context.Context, method, path string, query url.Values, body any) ([]byte, http.Header, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot encode the request body: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	target := c.baseURL + path
	if len(query) != 0 {
		target += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "application/json, application/yaml")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.authenticated.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read the answer of %s: %w", req.URL, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, nil, rawError(resp, raw)
	}
	return raw, resp.Header.Clone(), nil
}

func rawError(resp *http.Response, raw []byte) error {
	message := ""
	var envelope api.Response[any]
	if json.Unmarshal(raw, &envelope) == nil {
		message = envelope.Message
	}
	if message == "" {
		var direct struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		if json.Unmarshal(raw, &direct) == nil {
			if direct.Message != "" {
				message = direct.Message
			} else {
				message = direct.Error
			}
		}
	}
	if message == "" {
		message = strings.TrimSpace(string(raw))
		if len(message) > messageLimit {
			message = message[:messageLimit] + "..."
		}
	}
	return &api.Error{StatusCode: resp.StatusCode, Message: message, Challenge: resp.Header.Get("WWW-Authenticate")}
}

func (c *Client) tenantObjectPath(group, version, tenant, resource, name string) string {
	return "/" + group + "/" + version + "/tenants/" + url.PathEscape(tenant) + "/" + resource + "/" + url.PathEscape(name)
}

func (c *Client) cloudShellSessionsPath(tenant string) string {
	return "/cs/v1alpha1/tenants/" + url.PathEscape(tenant) + "/sessions"
}
