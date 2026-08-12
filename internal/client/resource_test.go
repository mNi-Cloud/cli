package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mNi-Cloud/cli/internal/api"
	"github.com/mNi-Cloud/cli/internal/unstructured"
)

type recordedCall struct {
	method      string
	path        string
	contentType string
	body        map[string]any
}

func newResourceClient(t *testing.T, resource api.APIResource, tenant string, handle func(w http.ResponseWriter, r *http.Request)) (ResourceClient, *[]recordedCall) {
	t.Helper()

	calls := &[]recordedCall{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("ReadAll() error = %v", err)
		}
		call := recordedCall{method: r.Method, path: r.URL.Path, contentType: r.Header.Get("Content-Type")}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &call.body); err != nil {
				t.Errorf("Unmarshal() error = %v", err)
			}
		}
		*calls = append(*calls, call)
		handle(w, r)
	}))
	t.Cleanup(server.Close)

	client := newTestClient(t, server, &staticTokens{token: "access"})
	resourceClient, err := client.Resource(resource, tenant)
	if err != nil {
		t.Fatalf("Resource() error = %v", err)
	}
	return resourceClient, calls
}

func TestResourceClientList(t *testing.T) {
	resource, calls := newResourceClient(t, namespacedResource, "e2etest", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusOK, api.Response[unstructured.UnstructuredList]{
			Success: true,
			Data:    unstructured.UnstructuredList{{"metadata": map[string]any{"name": "a"}}},
		})
	})

	list, err := resource.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 || list[0].Name() != "a" {
		t.Errorf("List() = %+v, want one resource named a", list)
	}
	if (*calls)[0].method != http.MethodGet {
		t.Errorf("method = %q, want GET", (*calls)[0].method)
	}
	if (*calls)[0].path != "/vpc/v1alpha2/tenants/e2etest/vpcs" {
		t.Errorf("path = %q, want the collection path", (*calls)[0].path)
	}
}

func TestResourceClientGet(t *testing.T) {
	resource, calls := newResourceClient(t, namespacedResource, "e2etest", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusOK, api.Response[unstructured.Unstructured]{
			Success: true,
			Data:    unstructured.Unstructured{"metadata": map[string]any{"name": "a"}},
		})
	})

	got, err := resource.Get(context.Background(), "a")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name() != "a" {
		t.Errorf("Name() = %q, want %q", got.Name(), "a")
	}
	if (*calls)[0].path != "/vpc/v1alpha2/tenants/e2etest/vpcs/a" {
		t.Errorf("path = %q, want the object path", (*calls)[0].path)
	}
}

func TestResourceClientCreate(t *testing.T) {
	resource, calls := newResourceClient(t, namespacedResource, "e2etest", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusCreated, api.Response[unstructured.Unstructured]{
			Success: true,
			Data:    unstructured.Unstructured{"metadata": map[string]any{"name": "a"}},
		})
	})

	if _, err := resource.Create(context.Background(), unstructured.Unstructured{"kind": "Vpc"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	call := (*calls)[0]
	if call.method != http.MethodPost {
		t.Errorf("method = %q, want POST", call.method)
	}
	if call.path != "/vpc/v1alpha2/tenants/e2etest/vpcs" {
		t.Errorf("path = %q, want the collection path", call.path)
	}
	if call.contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", call.contentType)
	}
	if call.body["kind"] != "Vpc" {
		t.Errorf("body = %+v, want the manifest that was passed", call.body)
	}
}

func TestResourceClientUpdate(t *testing.T) {
	resource, calls := newResourceClient(t, namespacedResource, "e2etest", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusOK, api.Response[unstructured.Unstructured]{
			Success: true,
			Data:    unstructured.Unstructured{"metadata": map[string]any{"name": "a"}},
		})
	})

	if _, err := resource.Update(context.Background(), "a", unstructured.Unstructured{"kind": "Vpc"}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	call := (*calls)[0]
	if call.method != http.MethodPut {
		t.Errorf("method = %q, want PUT", call.method)
	}
	if call.path != "/vpc/v1alpha2/tenants/e2etest/vpcs/a" {
		t.Errorf("path = %q, want the object path", call.path)
	}
}

func TestResourceClientPatchContentTypes(t *testing.T) {
	tests := []struct {
		patchType PatchType
		want      string
	}{
		{patchType: MergePatchType, want: "application/merge-patch+json"},
		{patchType: ApplyPatchType, want: "application/apply-patch+json"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			resource, calls := newResourceClient(t, namespacedResource, "e2etest", func(w http.ResponseWriter, r *http.Request) {
				writeEnvelope(t, w, http.StatusOK, api.Response[unstructured.Unstructured]{
					Success: true,
					Data:    unstructured.Unstructured{"metadata": map[string]any{"name": "a"}},
				})
			})

			if _, err := resource.Patch(context.Background(), "a", tt.patchType, unstructured.Unstructured{"kind": "Vpc"}); err != nil {
				t.Fatalf("Patch() error = %v", err)
			}

			call := (*calls)[0]
			if call.method != http.MethodPatch {
				t.Errorf("method = %q, want PATCH", call.method)
			}
			if call.contentType != tt.want {
				t.Errorf("Content-Type = %q, want %q", call.contentType, tt.want)
			}
		})
	}
}

func TestResourceClientPatchRejectsAnUnknownType(t *testing.T) {
	resource, _ := newResourceClient(t, namespacedResource, "e2etest", func(w http.ResponseWriter, r *http.Request) {})

	if _, err := resource.Patch(context.Background(), "a", PatchType(42), unstructured.Unstructured{}); err == nil {
		t.Fatal("Patch() error = nil, want an unknown patch type refused")
	}
}

func TestResourceClientDelete(t *testing.T) {
	resource, calls := newResourceClient(t, namespacedResource, "e2etest", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusOK, api.Response[any]{Success: true})
	})

	if err := resource.Delete(context.Background(), "a"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	call := (*calls)[0]
	if call.method != http.MethodDelete {
		t.Errorf("method = %q, want DELETE", call.method)
	}
	if call.path != "/vpc/v1alpha2/tenants/e2etest/vpcs/a" {
		t.Errorf("path = %q, want the object path", call.path)
	}
}

func TestResourceClientDeleteReportsANotFound(t *testing.T) {
	resource, _ := newResourceClient(t, namespacedResource, "e2etest", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, http.StatusNotFound, api.Response[any]{Success: false, Message: "not found"})
	})

	if err := resource.Delete(context.Background(), "a"); !api.IsNotFound(err) {
		t.Fatalf("Delete() error = %v, want a 404", err)
	}
}

func TestResourceClientRejectsAnEmptyName(t *testing.T) {
	resource, _ := newResourceClient(t, namespacedResource, "e2etest", func(w http.ResponseWriter, r *http.Request) {})

	if _, err := resource.Get(context.Background(), ""); err == nil {
		t.Error("Get(\"\") error = nil, want an error")
	}
	if err := resource.Delete(context.Background(), ""); err == nil {
		t.Error("Delete(\"\") error = nil, want an error")
	}
}
