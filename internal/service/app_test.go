package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"gittar/internal/config"
)

type mockNotifier struct {
	alerts []string
}

func (m *mockNotifier) Notify(title, body string) error {
	m.alerts = append(m.alerts, fmt.Sprintf("%s: %s", title, body))
	return nil
}

func (m *mockNotifier) UpdateTray(passing, failing, running int) {}

func TestPipelineTransitions(t *testing.T) {
	// Set up temporary config directory
	tmpDir, err := os.MkdirTemp("", "gittar-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// Override HOME environment variable to isolate config dir
	oldHome := os.Getenv("HOME")
	defer func() {
		_ = os.Setenv("HOME", oldHome)
	}()
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("failed to set HOME env var: %v", err)
	}

	// Create a valid config file
	conf := &config.Config{
		GitLabURL:         "http://example.com",
		Token:             "mock-token",
		MonitoredProjects: []string{"group/project1"},
		Notifications: config.NotificationSettings{
			Enabled:         true,
			PipelineSuccess: true,
			PipelineFailed:  true,
		},
	}
	err = config.SaveConfig(conf)
	if err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Dynamic pipeline returned by mock server
	var currentPipelineID int32 = 100
	var currentPipelineStatus = "running"
	var currentPipelineRef = "main"

	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		w.Header().Set("Content-Type", "application/json")

		if path == "/api/v4/user" {
			_, _ = w.Write([]byte(`{"id": 5, "username": "testuser"}`))
			return
		}
		if path == "/api/v4/todos" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if strings.HasPrefix(path, "/api/v4/merge_requests") {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if strings.HasPrefix(path, "/api/v4/issues") {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if path == "/api/v4/projects/group/project1" {
			_, _ = w.Write([]byte(`{"id": 1, "name": "project1", "path_with_namespace": "group/project1"}`))
			return
		}
		if path == "/api/v4/projects/group/project1/merge_requests" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if path == "/api/v4/projects/group/project1/issues" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if path == "/api/v4/projects/group/project1/pipelines" {
			pid := atomic.LoadInt32(&currentPipelineID)
			ref := currentPipelineRef
			_, _ = fmt.Fprintf(w, `[{"id": %d, "ref": "%s", "status": "running"}]`, pid, ref)
			return
		}
		if strings.HasPrefix(path, "/api/v4/projects/group/project1/pipelines/") {
			pid := atomic.LoadInt32(&currentPipelineID)
			status := currentPipelineStatus
			ref := currentPipelineRef
			if strings.HasSuffix(path, "/jobs") {
				_, _ = w.Write([]byte(`[]`))
			} else {
				_, _ = fmt.Fprintf(w, `{"id": %d, "ref": "%s", "status": "%s"}`, pid, ref, status)
			}
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Update config to use the mock server URL
	conf.GitLabURL = server.URL
	_ = config.SaveConfig(conf)

	// Create AppService and setup mock notifier
	appService := NewAppService()
	notifier := &mockNotifier{}
	appService.SetTray(notifier)

	// --- Step 1: First fetch ---
	// Pipeline is ID 100, status "running".
	appService.ClearTelemetryCache()
	_, err = appService.FetchTelemetry()
	if err != nil {
		t.Fatalf("unexpected fetch error: %v", err)
	}
	// First fetch should not trigger notifications
	if len(notifier.alerts) != 0 {
		t.Errorf("expected 0 alerts on first fetch, got %v", notifier.alerts)
	}

	// --- Step 2: Same pipeline transitions to "success" ---
	currentPipelineStatus = "success"
	notifier.alerts = nil // reset alerts
	appService.ClearTelemetryCache()
	_, err = appService.FetchTelemetry()
	if err != nil {
		t.Fatalf("unexpected fetch error: %v", err)
	}
	if len(notifier.alerts) != 1 || !strings.Contains(notifier.alerts[0], "Pipeline Passed") {
		t.Errorf("expected 1 'Pipeline Passed' alert, got %v", notifier.alerts)
	}

	// --- Step 3: Fetch again with same success status ---
	// Should not alert again (duplicate prevention)
	notifier.alerts = nil
	appService.ClearTelemetryCache()
	_, err = appService.FetchTelemetry()
	if err != nil {
		t.Fatalf("unexpected fetch error: %v", err)
	}
	if len(notifier.alerts) != 0 {
		t.Errorf("expected 0 alerts for same status, got %v", notifier.alerts)
	}

	// --- Step 4: New pipeline run (ID 101) succeeds immediately ---
	// Should alert on different ID even if status was already success
	atomic.StoreInt32(&currentPipelineID, 101)
	currentPipelineStatus = "success"
	notifier.alerts = nil
	appService.ClearTelemetryCache()
	_, err = appService.FetchTelemetry()
	if err != nil {
		t.Fatalf("unexpected fetch error: %v", err)
	}
	if len(notifier.alerts) != 1 || !strings.Contains(notifier.alerts[0], "Pipeline Passed") {
		t.Errorf("expected 1 'Pipeline Passed' alert for new pipeline run, got %v", notifier.alerts)
	}

	// --- Step 5: Test branch separation ---
	// A run on another branch "feature" fails.
	atomic.StoreInt32(&currentPipelineID, 102)
	currentPipelineRef = "feature"
	currentPipelineStatus = "failed"
	notifier.alerts = nil

	appService.ClearTelemetryCache()
	_, err = appService.FetchTelemetry()
	if err != nil {
		t.Fatalf("unexpected fetch error: %v", err)
	}
	// Note: since "feature" is a new branch key ("group/project1:feature"),
	// its first fetch should NOT trigger a notification because exists is false for that key.
	if len(notifier.alerts) != 0 {
		t.Errorf("expected 0 alerts on first fetch of new feature branch, got %v", notifier.alerts)
	}

	// Next fetch on "feature" branch completes transition (still failed)
	appService.ClearTelemetryCache()
	_, err = appService.FetchTelemetry()
	if err != nil {
		t.Fatalf("unexpected fetch error: %v", err)
	}
	// Since status is failed, it should not alert if status hasn't transitioned,
	// BUT wait, it's the same run (ID 102) and same status (failed). So 0 alerts.
	if len(notifier.alerts) != 0 {
		t.Errorf("expected 0 alerts when status doesn't change on feature branch, got %v", notifier.alerts)
	}

	// Now feature branch transitions to success
	currentPipelineStatus = "success"
	notifier.alerts = nil
	appService.ClearTelemetryCache()
	_, err = appService.FetchTelemetry()
	if err != nil {
		t.Fatalf("unexpected fetch error: %v", err)
	}
	if len(notifier.alerts) != 1 || !strings.Contains(notifier.alerts[0], "Pipeline Passed") {
		t.Errorf("expected 1 'Pipeline Passed' alert for feature branch, got %v", notifier.alerts)
	}
}

func TestIssues(t *testing.T) {
	// Set up temporary config directory
	tmpDir, err := os.MkdirTemp("", "gittar-issues-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// Override HOME environment variable to isolate config dir
	oldHome := os.Getenv("HOME")
	defer func() {
		_ = os.Setenv("HOME", oldHome)
	}()
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("failed to set HOME env var: %v", err)
	}

	// Create a valid config file
	conf := &config.Config{
		GitLabURL:         "http://example.com",
		Token:             "mock-token",
		MonitoredProjects: []string{"group/project1"},
	}
	err = config.SaveConfig(conf)
	if err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	var issueClosed bool

	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		w.Header().Set("Content-Type", "application/json")

		if path == "/api/v4/user" {
			_, _ = w.Write([]byte(`{"id": 5, "username": "testuser"}`))
			return
		}
		if path == "/api/v4/todos" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if strings.HasPrefix(path, "/api/v4/merge_requests") {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if path == "/api/v4/projects/group/project1" {
			_, _ = w.Write([]byte(`{"id": 1, "name": "project1", "path_with_namespace": "group/project1"}`))
			return
		}
		if path == "/api/v4/projects/group/project1/merge_requests" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if path == "/api/v4/projects/group/project1/pipelines" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if path == "/api/v4/issues" {
			_, _ = w.Write([]byte(`[{"id": 10, "iid": 100, "project_id": 1, "title": "User Issue 1", "state": "opened", "web_url": "http://example.com/group/project1/-/issues/100"}]`))
			return
		}
		if path == "/api/v4/projects/group/project1/issues" {
			_, _ = w.Write([]byte(`[{"id": 11, "iid": 101, "project_id": 1, "title": "Project Issue 1", "state": "opened", "web_url": "http://example.com/group/project1/-/issues/101"}]`))
			return
		}
		if path == "/api/v4/projects/1/issues/100" && r.Method == "PUT" {
			issueClosed = true
			_, _ = w.Write([]byte(`{"id": 10, "iid": 100, "project_id": 1, "title": "User Issue 1", "state": "closed"}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Update config to use the mock server URL
	conf.GitLabURL = server.URL
	_ = config.SaveConfig(conf)

	// Create AppService
	appService := NewAppService()

	appService.ClearTelemetryCache()
	payload, err := appService.FetchTelemetry()
	if err != nil {
		t.Fatalf("unexpected fetch error: %v", err)
	}

	if len(payload.Issues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(payload.Issues))
	}

	// Verify details of the first issue
	foundUserIssue := false
	foundProjIssue := false
	for _, issue := range payload.Issues {
		if issue.Title == "User Issue 1" {
			foundUserIssue = true
		}
		if issue.Title == "Project Issue 1" {
			foundProjIssue = true
		}
	}

	if !foundUserIssue || !foundProjIssue {
		t.Errorf("did not find expected issues: user_issue=%t, proj_issue=%t", foundUserIssue, foundProjIssue)
	}

	// Test closing the issue
	err = appService.CloseIssue(1, 100)
	if err != nil {
		t.Fatalf("failed to close issue: %v", err)
	}

	if !issueClosed {
		t.Errorf("issue was not marked closed on mock server")
	}
}
