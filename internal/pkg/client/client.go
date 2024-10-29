package client

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/labstack/gommon/log"
	"io"
	"net/http"
	"strings"
)

type (
	Response struct {
		StatusCode int
		*SingletonResponse
		*MultitonResponse
	}

	SingletonResponse struct {
		Body map[string]interface{}
	}

	MultitonResponse struct {
		Body []map[string]interface{}
	}

	ErrorResponse struct {
		Message string `json:"message"`
	}

	Params struct {
		Authorization string
		XNamespace    *string
	}

	RestClientInterface interface {
		Get(ctx context.Context, name string, params *Params) (*Response, error)
		List(ctx context.Context, params *Params) (*Response, error)
		Create(ctx context.Context, obj map[string]interface{}, params *Params) (*Response, error)
		Patch(ctx context.Context, name string, obj map[string]interface{}, params *Params) (*Response, error)
		Delete(ctx context.Context, name string, params *Params) (*Response, error)
	}

	RestClient struct {
		rootUrl      string
		apiVersion   string
		resourceName string

		client *http.Client
	}
)

func (r *RestClient) Get(ctx context.Context, name string, params *Params) (*Response, error) {
	req, err := http.NewRequest("GET", r.getUrl()+"/"+name, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", params.Authorization)
	if params.XNamespace != nil {
		req.Header.Set("X-namespace", *params.XNamespace)
	}

	return r.processRequest(ctx, req)
}

func (r *RestClient) List(ctx context.Context, params *Params) (*Response, error) {
	req, err := http.NewRequest("GET", r.getUrl(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", params.Authorization)
	if params.XNamespace != nil {
		req.Header.Set("X-namespace", *params.XNamespace)
	}

	return r.processRequest(ctx, req)
}

func (r *RestClient) Create(ctx context.Context, obj map[string]interface{}, params *Params) (*Response, error) {
	byteArray, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", r.getUrl(), bytes.NewBuffer(byteArray))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", params.Authorization)
	if params.XNamespace != nil {
		req.Header.Set("X-namespace", *params.XNamespace)
	}

	return r.processRequest(ctx, req)
}

func (r *RestClient) Patch(ctx context.Context, name string, obj map[string]interface{}, params *Params) (*Response, error) {
	byteArray, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PATCH", r.getUrl()+"/"+name, bytes.NewBuffer(byteArray))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", params.Authorization)
	if params.XNamespace != nil {
		req.Header.Set("X-namespace", *params.XNamespace)
	}

	return r.processRequest(ctx, req)
}

func (r *RestClient) Delete(ctx context.Context, name string, params *Params) (*Response, error) {
	req, err := http.NewRequest("DELETE", r.getUrl()+"/"+name, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", params.Authorization)
	if params.XNamespace != nil {
		req.Header.Set("X-namespace", *params.XNamespace)
	}

	return r.processRequest(ctx, req)
}

func (r *RestClient) processRequest(ctx context.Context, req *http.Request) (*Response, error) {
	res, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Debugf("Error closing response body: %s", err.Error())
		}
	}(res.Body)

	response := &Response{
		StatusCode: res.StatusCode,
	}

	if strings.Contains(res.Header.Get("Content-Type"), "application/json") {
		byteArray, err := io.ReadAll(res.Body)
		if err != nil {
			log.Debugf("Error reading response body: %s", err.Error())
			return nil, err
		}

		var obj map[string]interface{}
		if err := json.Unmarshal(byteArray, &obj); err != nil {
			var objList []map[string]interface{}
			if err2 := json.Unmarshal(byteArray, &objList); err2 != nil {
				log.Debugf("Error unmarshalling response body: %s, %s", err.Error(), err2.Error())
				return nil, err
			}
			response.MultitonResponse = &MultitonResponse{
				Body: objList,
			}
		} else {
			response.SingletonResponse = &SingletonResponse{
				Body: obj,
			}
		}
	}

	return response, nil
}

func (r *RestClient) getUrl() string {
	if !strings.HasSuffix(r.rootUrl, "/") {
		r.rootUrl += "/"
	}
	return r.rootUrl + r.apiVersion + "/" + r.resourceName
}

func NewRestClient(rootUrl, apiVersion, resourceName string) RestClientInterface {
	return &RestClient{
		rootUrl:      rootUrl,
		apiVersion:   apiVersion,
		resourceName: resourceName,

		client: new(http.Client),
	}
}
