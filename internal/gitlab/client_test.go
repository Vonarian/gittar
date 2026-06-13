package gitlab

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMarkTodoAsDone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify endpoint path
		if r.URL.Path != "/api/v4/todos/123/mark_as_done" {
			t.Errorf("expected path /api/v4/todos/123/mark_as_done, got %s", r.URL.Path)
		}
		// Verify method
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		// Verify auth header
		if r.Header.Get("PRIVATE-TOKEN") != "test-token" {
			t.Errorf("expected PRIVATE-TOKEN 'test-token', got %s", r.Header.Get("PRIVATE-TOKEN"))
		}

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id": 123, "state": "done"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", nil)
	err := client.MarkTodoAsDone(123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRetryPipeline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify endpoint path
		if r.RequestURI != "/api/v4/projects/group%2Fproject-name/pipelines/456/retry" {
			t.Errorf("expected RequestURI /api/v4/projects/group%%2Fproject-name/pipelines/456/retry, got %s", r.RequestURI)
		}
		// Verify method
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id": 456, "status": "pending"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", nil)
	err := client.RetryPipeline("group/project-name", 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCancelPipeline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify endpoint path
		if r.RequestURI != "/api/v4/projects/group%2Fproject-name/pipelines/789/cancel" {
			t.Errorf("expected RequestURI /api/v4/projects/group%%2Fproject-name/pipelines/789/cancel, got %s", r.RequestURI)
		}
		// Verify method
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id": 789, "status": "canceled"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", nil)
	err := client.CancelPipeline("group/project-name", 789)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewClientWithProxy(t *testing.T) {
	proxyConf := &ProxyConfig{
		Enabled:  true,
		Host:     "127.0.0.1",
		Port:     1080,
		User:     "user",
		Password: "password",
	}

	client := NewClient("https://gitlab.example.com", "test-token", proxyConf)
	if client.BaseURL != "https://gitlab.example.com" {
		t.Errorf("expected BaseURL https://gitlab.example.com, got %s", client.BaseURL)
	}

	// Verify that DialContext function is configured on the transport
	transport, ok := client.HTTPClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected Transport to be *http.Transport")
	}

	if transport.DialContext == nil {
		t.Errorf("expected DialContext function to be configured when proxy is enabled")
	}
}

func TestGetGroupProjects(t *testing.T) {
	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// Verify endpoint path (escaped group name "my-group")
		if r.URL.Path != "/api/v4/groups/my-group/projects" {
			t.Errorf("expected path /api/v4/groups/my-group/projects, got %s", r.URL.Path)
		}

		query := r.URL.Query()
		if query.Get("simple") != "true" {
			t.Errorf("expected simple=true query param, got %s", query.Get("simple"))
		}
		if query.Get("include_subgroups") != "true" {
			t.Errorf("expected include_subgroups=true query param, got %s", query.Get("include_subgroups"))
		}
		if query.Get("per_page") != "100" {
			t.Errorf("expected per_page=100 query param, got %s", query.Get("per_page"))
		}

		page := query.Get("page")
		w.Header().Set("Content-Type", "application/json")
		switch page {
		case "1":
			// Return 100 items (representing first full page)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[` + strings.Join(generateJSONProjects(100), ",") + `]`))
		case "2":
			// Return 2 items (representing last partial page)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[` + strings.Join(generateJSONProjects(2), ",") + `]`))
		default:
			t.Errorf("unexpected page request: %s", page)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("[]"))
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", nil)
	projects, err := client.GetGroupProjects("my-group")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects) != 102 {
		t.Errorf("expected 102 projects, got %d", len(projects))
	}
	if callCount != 2 {
		t.Errorf("expected 2 page requests, got %d", callCount)
	}
}

func generateJSONProjects(count int) []string {
	var out []string
	for i := 0; i < count; i++ {
		out = append(out, fmt.Sprintf(`{"id": %d, "name": "project-%d", "path_with_namespace": "my-group/project-%d"}`, i, i, i))
	}
	return out
}

