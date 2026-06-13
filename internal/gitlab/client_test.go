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

func TestGetProjectMergeRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if path == "/api/v4/projects/group/project1/merge_requests" {
			if r.URL.Query().Get("state") != "opened" {
				t.Errorf("expected state=opened query param, got %s", r.URL.Query().Get("state"))
			}
			_, _ = w.Write([]byte(`[{"id": 1, "iid": 101, "project_id": 10, "title": "Test MR 1", "state": "opened", "web_url": "http://example.com/group/project1/-/merge_requests/101"}]`))
			return
		}

		if path == "/api/v4/projects/10/merge_requests/101" {
			_, _ = w.Write([]byte(`{"id": 1, "iid": 101, "project_id": 10, "title": "Test MR 1", "state": "opened", "web_url": "http://example.com/group/project1/-/merge_requests/101"}`))
			return
		}

		t.Errorf("unexpected path: %s", path)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", nil)
	mrs, err := client.GetProjectMergeRequests("group/project1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mrs) != 1 {
		t.Fatalf("expected 1 MR, got %d", len(mrs))
	}
	if mrs[0].Title != "Test MR 1" {
		t.Errorf("expected title 'Test MR 1', got %s", mrs[0].Title)
	}
}

func TestGetPipelinesWithJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if path == "/api/v4/projects/group/project1" {
			_, _ = w.Write([]byte(`{"id": 10, "name": "project1", "path_with_namespace": "group/project1"}`))
			return
		}
		if path == "/api/v4/projects/group/project1/pipelines" {
			_, _ = w.Write([]byte(`[{"id": 201, "ref": "main", "status": "success"}]`))
			return
		}
		if path == "/api/v4/projects/group/project1/pipelines/201" {
			_, _ = w.Write([]byte(`{"id": 201, "ref": "main", "status": "success", "duration": 120}`))
			return
		}
		if path == "/api/v4/projects/group/project1/pipelines/201/jobs" {
			_, _ = w.Write([]byte(`[{"id": 301, "name": "build", "stage": "build", "status": "success"}]`))
			return
		}

		t.Errorf("unexpected path: %s", path)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", nil)
	pwjList, err := client.GetPipelinesWithJobs("group/project1", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pwjList) != 1 {
		t.Fatalf("expected 1 PipelineWithJobs, got %d", len(pwjList))
	}
	pwj := pwjList[0]
	if pwj.ProjectName != "project1" {
		t.Errorf("expected ProjectName 'project1', got %s", pwj.ProjectName)
	}
	if pwj.Pipeline.ID != 201 {
		t.Errorf("expected Pipeline ID 201, got %d", pwj.Pipeline.ID)
	}
	if len(pwj.Jobs) != 1 || pwj.Jobs[0].Name != "build" {
		t.Errorf("expected 1 job named 'build', got %v", pwj.Jobs)
	}
}

