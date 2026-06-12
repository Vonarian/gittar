package gitlab

import (
	"net/http"
	"net/http/httptest"
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

