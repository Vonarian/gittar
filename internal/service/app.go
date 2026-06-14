package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gittar/internal/config"
	"gittar/internal/gitlab"
)

// Notifier defines the interface for sending system alerts and updating the tray state.
type Notifier interface {
	Notify(title, body string) error
	UpdateTray(passing, failing, running int)
}

// pipelineState tracks the ID and status of a pipeline run.
type pipelineState struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
}

// mrState tracks the state and head pipeline status of a Merge Request.
type mrState struct {
	ID             int    `json:"id"`
	IID            int    `json:"iid"`
	ProjectID      int    `json:"project_id"`
	Title          string `json:"title"`
	State          string `json:"state"`
	PipelineStatus string `json:"pipeline_status"`
}

func cleanRef(ref string) string {
	if strings.HasPrefix(ref, "refs/heads/") {
		return strings.TrimPrefix(ref, "refs/heads/")
	}
	if strings.HasPrefix(ref, "refs/merge-requests/") {
		parts := strings.Split(ref, "/")
		if len(parts) >= 3 {
			return fmt.Sprintf("MR !%s", parts[2])
		}
	}
	return ref
}

func getPipelineStatus(hp *gitlab.HeadPipeline) string {
	if hp == nil {
		return ""
	}
	return hp.Status
}

// AppService handles bindings and telemetry orchestration.
type AppService struct {
	trayService    Notifier
	gitlabClient   *gitlab.Client
	gitlabURL      string
	gitlabToken    string
	pipelineStates map[string]pipelineState
	seenTodoIDs    map[int]bool
	seenMRIDs      map[int]bool
	mrStates       map[int]mrState
	isFirstFetch   bool
	stateMu        sync.Mutex
	proxyEnabled   bool
	proxyHost      string
	proxyPort      int
	proxyUser      string
	proxyPassword  string
}

// NewAppService creates a new application service instance.
func NewAppService() *AppService {
	return &AppService{
		pipelineStates: make(map[string]pipelineState),
		seenTodoIDs:    make(map[int]bool),
		seenMRIDs:      make(map[int]bool),
		mrStates:       make(map[int]mrState),
		isFirstFetch:   true,
	}
}

// GetCachedTelemetry reads the cached GitLab telemetry payload from disk.
func (s *AppService) GetCachedTelemetry() (*gitlab.TelemetryPayload, error) {
	dir, err := config.GetConfigDir()
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(dir, "cache.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var payload gitlab.TelemetryPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}

	return &payload, nil
}


// getGitLabClient retrieves the cached GitLab client or creates one if the config changed.
func (s *AppService) getGitLabClient(conf *config.Config) *gitlab.Client {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	if s.gitlabClient == nil ||
		s.gitlabURL != conf.GitLabURL ||
		s.gitlabToken != conf.Token ||
		s.proxyEnabled != conf.ProxyEnabled ||
		s.proxyHost != conf.ProxyHost ||
		s.proxyPort != conf.ProxyPort ||
		s.proxyUser != conf.ProxyUser ||
		s.proxyPassword != conf.ProxyPassword {

		// Close idle connections on the old client's transport to avoid leaking/reusing unproxied connections
		if s.gitlabClient != nil && s.gitlabClient.HTTPClient != nil {
			if t, ok := s.gitlabClient.HTTPClient.Transport.(*http.Transport); ok {
				t.CloseIdleConnections()
			}
		}

		proxyConf := &gitlab.ProxyConfig{
			Enabled:  conf.ProxyEnabled,
			Host:     conf.ProxyHost,
			Port:     conf.ProxyPort,
			User:     conf.ProxyUser,
			Password: conf.ProxyPassword,
		}

		s.gitlabClient = gitlab.NewClient(conf.GitLabURL, conf.Token, proxyConf)
		s.gitlabURL = conf.GitLabURL
		s.gitlabToken = conf.Token
		s.proxyEnabled = conf.ProxyEnabled
		s.proxyHost = conf.ProxyHost
		s.proxyPort = conf.ProxyPort
		s.proxyUser = conf.ProxyUser
		s.proxyPassword = conf.ProxyPassword
	}
	return s.gitlabClient
}

//wails:ignore
// SetTray links the system tray manager to the application service.
func (s *AppService) SetTray(t Notifier) {
	s.trayService = t
}

// GetConfig reads the user config.
func (s *AppService) GetConfig() (*config.Config, error) {
	return config.LoadConfig()
}

// SaveConfig saves the user config.
func (s *AppService) SaveConfig(conf *config.Config) error {
	return config.SaveConfig(conf)
}

// GetJobLogSnippet fetches log tail for a specific failed job.
func (s *AppService) GetJobLogSnippet(projectIDOrPath string, jobID int) (string, error) {
	conf, err := config.LoadConfig()
	if err != nil {
		return "", err
	}
	if conf.Token == "" {
		return "", fmt.Errorf("GitLab token not configured")
	}

	client := s.getGitLabClient(conf)
	return client.GetJobLogSnippet(projectIDOrPath, jobID)
}

// FetchTelemetry fetches all telemetry data concurrently.
func (s *AppService) FetchTelemetry() (*gitlab.TelemetryPayload, error) {
	conf, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}
	if conf.Token == "" {
		return &gitlab.TelemetryPayload{Error: "unconfigured"}, nil
	}

	client := s.getGitLabClient(conf)

	// 1. Fetch User details to verify connection and get ID
	user, err := client.GetCurrentUser()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to GitLab: %w", err)
	}

	// 2. Resolve Monitored Projects (direct + group resolution)
	dedupProjects := make(map[string]bool)
	for _, p := range conf.MonitoredProjects {
		if p != "" {
			dedupProjects[p] = true
		}
	}

	// Fetch projects under monitored groups
	var groupProjMu sync.Mutex
	var groupWg sync.WaitGroup
	for _, g := range conf.MonitoredGroups {
		if g == "" {
			continue
		}
		groupWg.Add(1)
		go func(groupID string) {
			defer groupWg.Done()
			projects, err := client.GetGroupProjects(groupID)
			if err == nil {
				groupProjMu.Lock()
				for _, p := range projects {
					dedupProjects[p.PathWithNamespace] = true
				}
				groupProjMu.Unlock()
			}
		}(g)
	}
	groupWg.Wait()

	// 3. Fetch all telemetry in parallel
	var wg sync.WaitGroup
	var todos []gitlab.Todo
	var userMRs []gitlab.MergeRequest
	var userIssues []gitlab.Issue
	var todosErr, userMRsErr, userIssuesErr error

	wg.Add(3)
	go func() {
		defer wg.Done()
		todos, todosErr = client.GetTodos()
	}()
	go func() {
		defer wg.Done()
		userMRs, userMRsErr = client.GetMergeRequests(user.ID)
	}()
	go func() {
		defer wg.Done()
		userIssues, userIssuesErr = client.GetIssues(user.ID)
	}()

	// Fetch project-level pipelines, open MRs, and open issues concurrently
	var pipeMu sync.Mutex
	pipelines := make([]gitlab.PipelineWithJobs, 0)

	var projMRsMu sync.Mutex
	var projMRs []gitlab.MergeRequest

	var projIssuesMu sync.Mutex
	var projIssues []gitlab.Issue

	for projectPath := range dedupProjects {
		// Fetch pipelines
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			pipes, err := client.GetPipelinesWithJobs(path, 10)
			if err == nil && len(pipes) > 0 {
				pipeMu.Lock()
				pipelines = append(pipelines, pipes...)
				pipeMu.Unlock()
			}
		}(projectPath)

		// Fetch project-level open MRs
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			mrs, err := client.GetProjectMergeRequests(path)
			if err == nil && len(mrs) > 0 {
				projMRsMu.Lock()
				projMRs = append(projMRs, mrs...)
				projMRsMu.Unlock()
			}
		}(projectPath)

		// Fetch project-level open issues
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			issues, err := client.GetProjectIssues(path)
			if err == nil && len(issues) > 0 {
				projIssuesMu.Lock()
				projIssues = append(projIssues, issues...)
				projIssuesMu.Unlock()
			}
		}(projectPath)
	}

	// Wait for all fetches to complete
	wg.Wait()

	if todosErr != nil {
		return nil, fmt.Errorf("failed to fetch todos: %w", todosErr)
	}
	if userMRsErr != nil {
		return nil, fmt.Errorf("failed to fetch merge requests: %w", userMRsErr)
	}
	if userIssuesErr != nil {
		return nil, fmt.Errorf("failed to fetch issues: %w", userIssuesErr)
	}

	// Deduplicate MRs between user-involved MRs and project-level MRs
	mrMap := make(map[int]gitlab.MergeRequest)
	for _, mr := range userMRs {
		mrMap[mr.ID] = mr
	}
	for _, mr := range projMRs {
		if _, exists := mrMap[mr.ID]; !exists {
			mrMap[mr.ID] = mr
		}
	}

	finalMRs := make([]gitlab.MergeRequest, 0, len(mrMap))
	for _, mr := range mrMap {
		finalMRs = append(finalMRs, mr)
	}

	// Sort merge requests by UpdatedAt descending (most recently updated first)
	sort.Slice(finalMRs, func(i, j int) bool {
		return finalMRs[i].UpdatedAt.After(finalMRs[j].UpdatedAt)
	})

	// Deduplicate Issues between user-involved Issues and project-level Issues
	issueMap := make(map[int]gitlab.Issue)
	for _, issue := range userIssues {
		issueMap[issue.ID] = issue
	}
	for _, issue := range projIssues {
		if _, exists := issueMap[issue.ID]; !exists {
			issueMap[issue.ID] = issue
		}
	}

	finalIssues := make([]gitlab.Issue, 0, len(issueMap))
	for _, issue := range issueMap {
		finalIssues = append(finalIssues, issue)
	}

	// Sort issues by UpdatedAt descending (most recently updated first)
	sort.Slice(finalIssues, func(i, j int) bool {
		return finalIssues[i].UpdatedAt.After(finalIssues[j].UpdatedAt)
	})

	// Sort pipelines alphabetically by projectName/path, and then by pipeline ID descending to keep newest first
	sort.Slice(pipelines, func(i, j int) bool {
		if pipelines[i].ProjectName == pipelines[j].ProjectName {
			if pipelines[i].ProjectPath == pipelines[j].ProjectPath {
				return pipelines[i].Pipeline.ID > pipelines[j].Pipeline.ID
			}
			return pipelines[i].ProjectPath < pipelines[j].ProjectPath
		}
		return pipelines[i].ProjectName < pipelines[j].ProjectName
	})

	// 4. Process pipeline transitions & trigger system alerts
	passingCount := 0
	failingCount := 0
	runningCount := 0

	processedKeys := make(map[string]bool)

	s.stateMu.Lock()
	for _, pwj := range pipelines {
		if pwj.Pipeline.ID == 0 {
			continue // No pipelines found for this project
		}

		status := pwj.Pipeline.Status
		path := pwj.ProjectPath
		name := pwj.ProjectName
		ref := pwj.Pipeline.Ref

		key := fmt.Sprintf("%s:%s", path, ref)
		if !processedKeys[key] {
			processedKeys[key] = true

			// Count statuses
			switch status {
			case "success":
				passingCount++
			case "failed":
				failingCount++
			case "running", "pending":
				runningCount++
			}

			// Check transition
			prev, exists := s.pipelineStates[key]
			if !s.isFirstFetch && conf.Notifications.Enabled {
				if !exists {
					// Brand new pipeline run/ref
					if status == "running" || status == "pending" {
						_ = s.trayService.Notify("Pipeline Started", fmt.Sprintf("%s: pipeline started on branch %s", name, cleanRef(ref)))
					}
				} else {
					isNewPipelineRun := prev.ID != pwj.Pipeline.ID
					isSamePipelineStatusChange := prev.ID == pwj.Pipeline.ID && prev.Status != status

					if isNewPipelineRun {
						if status == "running" || status == "pending" {
							_ = s.trayService.Notify("Pipeline Started", fmt.Sprintf("%s: pipeline started on branch %s", name, cleanRef(ref)))
						} else if status == "success" && conf.Notifications.PipelineSuccess {
							_ = s.trayService.Notify("Pipeline Passed", fmt.Sprintf("%s: pipeline passed on branch %s", name, cleanRef(ref)))
						} else if status == "failed" && conf.Notifications.PipelineFailed {
							_ = s.trayService.Notify("Pipeline Failed", fmt.Sprintf("%s: pipeline failed on branch %s", name, cleanRef(ref)))
						}
					} else if isSamePipelineStatusChange {
						if status == "success" && conf.Notifications.PipelineSuccess {
							_ = s.trayService.Notify("Pipeline Passed", fmt.Sprintf("%s: pipeline passed on branch %s", name, cleanRef(ref)))
						} else if status == "failed" && conf.Notifications.PipelineFailed {
							_ = s.trayService.Notify("Pipeline Failed", fmt.Sprintf("%s: pipeline failed on branch %s", name, cleanRef(ref)))
						}
					}
				}
			}
			s.pipelineStates[key] = pipelineState{
				ID:     pwj.Pipeline.ID,
				Status: status,
			}
		}
	}

	// 5. Process new Todos transitions
	for _, todo := range todos {
		if !s.seenTodoIDs[todo.ID] {
			s.seenTodoIDs[todo.ID] = true
			if !s.isFirstFetch && conf.Notifications.Enabled {
				action := todo.ActionName
				target := todo.TargetType
				body := todo.Body
				notify := false
				title := "Gittar Notification"

				if action == "mentioned" && conf.Notifications.TodoMention {
					title = "Mentioned in GitLab"
					notify = true
				} else if action == "assigned" {
					if target == "MergeRequest" && conf.Notifications.MRAssigned {
						title = "MR Assigned to You"
						notify = true
					} else if target == "Issue" && conf.Notifications.TodoAssignment {
						title = "Issue Assigned to You"
						notify = true
					} else if conf.Notifications.TodoAssignment {
						title = "Assigned in GitLab"
						notify = true
					}
				} else if target == "Issue" && conf.Notifications.TodoIssue {
					title = "New Issue Todo"
					notify = true
				} else if target == "MergeRequest" {
					// Handled by MR review/assignment logic below
				} else if conf.Notifications.TodoGeneric {
					title = fmt.Sprintf("GitLab Todo: %s", todo.ActionName)
					notify = true
				}

				if notify {
					_ = s.trayService.Notify(title, fmt.Sprintf("%s: %s", todo.Project.PathWithNamespace, body))
				}
			}
		}
	}

	// 6. Process new Merge Requests transitions
	if s.mrStates == nil {
		s.mrStates = make(map[int]mrState)
	}

	for _, mr := range finalMRs {
		newPipelineStatus := getPipelineStatus(mr.HeadPipeline)

		// Check if we already have a record for this MR
		prev, exists := s.mrStates[mr.ID]
		if exists && !s.isFirstFetch && conf.Notifications.Enabled {
			// 1. Check if MR's head pipeline status changed
			if prev.PipelineStatus != newPipelineStatus && newPipelineStatus != "" {
				if newPipelineStatus == "success" && conf.Notifications.PipelineSuccess {
					_ = s.trayService.Notify("MR Pipeline Passed", fmt.Sprintf("Pipeline passed for MR !%d: %s", mr.IID, mr.Title))
				} else if newPipelineStatus == "failed" && conf.Notifications.PipelineFailed {
					_ = s.trayService.Notify("MR Pipeline Failed", fmt.Sprintf("Pipeline failed for MR !%d: %s", mr.IID, mr.Title))
				}
			}
		}

		// Save the current state of the MR
		s.mrStates[mr.ID] = mrState{
			ID:             mr.ID,
			IID:            mr.IID,
			ProjectID:      mr.ProjectID,
			Title:          mr.Title,
			State:          mr.State,
			PipelineStatus: newPipelineStatus,
		}

		if !s.seenMRIDs[mr.ID] {
			s.seenMRIDs[mr.ID] = true
			if !s.isFirstFetch && conf.Notifications.Enabled {
				// Check if user is reviewer
				isReviewer := false
				for _, r := range mr.Reviewers {
					if r.Username == user.Username {
						isReviewer = true
						break
					}
				}

				// Check if user is assignee
				isAssignee := false
				for _, a := range mr.Assignees {
					if a.Username == user.Username {
						isAssignee = true
						break
					}
				}

				if isReviewer && conf.Notifications.MRReviewRequest {
					_ = s.trayService.Notify("Review Request", fmt.Sprintf("You are requested to review: %s", mr.Title))
				} else if isAssignee && conf.Notifications.MRAssigned {
					_ = s.trayService.Notify("MR Assigned", fmt.Sprintf("Merge Request assigned to you: %s", mr.Title))
				} else {
					_ = s.trayService.Notify("New Merge Request", fmt.Sprintf("New Merge Request !%d: %s", mr.IID, mr.Title))
				}
			}
		}
	}

	// 7. Check for closed/merged MRs asynchronously
	currentMRIDs := make(map[int]bool)
	for _, mr := range finalMRs {
		currentMRIDs[mr.ID] = true
	}

	for oldID, oldState := range s.mrStates {
		if !currentMRIDs[oldID] && oldState.State == "opened" {
			// This MR is no longer returned in the open list. Query its latest state in a background goroutine.
			go func(oState mrState, cl *gitlab.Client) {
				detailed, err := cl.GetSingleMergeRequest(oState.ProjectID, oState.IID)
				if err == nil && detailed != nil {
					s.stateMu.Lock()
					s.mrStates[detailed.ID] = mrState{
						ID:             detailed.ID,
						IID:            detailed.IID,
						ProjectID:      detailed.ProjectID,
						Title:          detailed.Title,
						State:          detailed.State,
						PipelineStatus: getPipelineStatus(detailed.HeadPipeline),
					}
					s.stateMu.Unlock()

					if !s.isFirstFetch && conf.Notifications.Enabled {
						switch detailed.State {
						case "merged":
							_ = s.trayService.Notify("MR Merged", fmt.Sprintf("Merge Request !%d was merged: %s", detailed.IID, oState.Title))
						case "closed":
							_ = s.trayService.Notify("MR Closed", fmt.Sprintf("Merge Request !%d was closed: %s", detailed.IID, oState.Title))
						}
					}
				}
			}(oldState, client)
		}
	}

	s.isFirstFetch = false

	// Update system tray label
	if s.trayService != nil {
		trayFailing := failingCount
		if conf != nil && conf.IgnoreFailedPipelines {
			trayFailing = 0
		}
		s.trayService.UpdateTray(passingCount, trayFailing, runningCount)
	}
	s.stateMu.Unlock()

	payload := &gitlab.TelemetryPayload{
		Todos:         todos,
		Pipelines:     pipelines,
		MergeRequests: finalMRs,
		Issues:        finalIssues,
		Username:      user.Username,
		AvatarURL:     user.AvatarURL,
	}

	// Save successfully fetched telemetry payload to disk cache asynchronously
	go func(p *gitlab.TelemetryPayload) {
		dir, err := config.GetConfigDir()
		if err != nil {
			return
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return
		}
		filePath := filepath.Join(dir, "cache.json")
		data, err := json.Marshal(p)
		if err != nil {
			return
		}
		_ = os.WriteFile(filePath, data, 0600)
	}(payload)

	return payload, nil
}

// MergeMergeRequest accepts/merges the GitLab MR.
func (s *AppService) MergeMergeRequest(projectID int, mrIID int) error {
	conf, err := config.LoadConfig()
	if err != nil {
		return err
	}
	if conf.Token == "" {
		return fmt.Errorf("GitLab token not configured")
	}

	client := s.getGitLabClient(conf)
	err = client.MergeMergeRequest(projectID, mrIID)
	if err == nil {
		s.ClearTelemetryCache()
	}
	return err
}

// CloseMergeRequest updates the GitLab MR state to closed.
func (s *AppService) CloseMergeRequest(projectID int, mrIID int) error {
	conf, err := config.LoadConfig()
	if err != nil {
		return err
	}
	if conf.Token == "" {
		return fmt.Errorf("GitLab token not configured")
	}

	client := s.getGitLabClient(conf)
	err = client.CloseMergeRequest(projectID, mrIID)
	if err == nil {
		s.ClearTelemetryCache()
	}
	return err
}

// CloseIssue updates the GitLab Issue state to closed.
func (s *AppService) CloseIssue(projectID int, issueIID int) error {
	conf, err := config.LoadConfig()
	if err != nil {
		return err
	}
	if conf.Token == "" {
		return fmt.Errorf("GitLab token not configured")
	}

	client := s.getGitLabClient(conf)
	err = client.CloseIssue(projectID, issueIID)
	if err == nil {
		s.ClearTelemetryCache()
	}
	return err
}

// MarkTodoAsDone marks the pending todo as done.
func (s *AppService) MarkTodoAsDone(todoID int) error {
	conf, err := config.LoadConfig()
	if err != nil {
		return err
	}
	if conf.Token == "" {
		return fmt.Errorf("GitLab token not configured")
	}

	client := s.getGitLabClient(conf)
	err = client.MarkTodoAsDone(todoID)
	if err == nil {
		s.ClearTelemetryCache()
	}
	return err
}

// RetryPipeline retries a failed pipeline.
func (s *AppService) RetryPipeline(projectPath string, pipelineID int) error {
	conf, err := config.LoadConfig()
	if err != nil {
		return err
	}
	if conf.Token == "" {
		return fmt.Errorf("GitLab token not configured")
	}

	client := s.getGitLabClient(conf)
	err = client.RetryPipeline(projectPath, pipelineID)
	if err == nil {
		s.ClearTelemetryCache()
	}
	return err
}

// CancelPipeline cancels a running pipeline.
func (s *AppService) CancelPipeline(projectPath string, pipelineID int) error {
	conf, err := config.LoadConfig()
	if err != nil {
		return err
	}
	if conf.Token == "" {
		return fmt.Errorf("GitLab token not configured")
	}

	client := s.getGitLabClient(conf)
	err = client.CancelPipeline(projectPath, pipelineID)
	if err == nil {
		s.ClearTelemetryCache()
	}
	return err
}

// TriggerTestNotification sends a test native notification.
func (s *AppService) TriggerTestNotification() error {
	if s.trayService == nil {
		return fmt.Errorf("tray service not initialized")
	}
	return s.trayService.Notify("Gittar Test", "This is a test notification from Gittar settings!")
}

// ClearTelemetryCache flushes the GitLab client cache.
func (s *AppService) ClearTelemetryCache() {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.gitlabClient != nil {
		s.gitlabClient.ClearCache()
	}
}


