package service

import (
	"fmt"
	"sync"

	"gittar/internal/config"
	"gittar/internal/gitlab"
	"gittar/internal/tray"
)

// AppService handles bindings and telemetry orchestration.
type AppService struct {
	trayService    *tray.TrayService
	pipelineStates map[string]string
	seenTodoIDs    map[int]bool
	seenMRIDs      map[int]bool
	isFirstFetch   bool
	stateMu        sync.Mutex
}

// NewAppService creates a new application service instance.
func NewAppService() *AppService {
	return &AppService{
		pipelineStates: make(map[string]string),
		seenTodoIDs:    make(map[int]bool),
		seenMRIDs:      make(map[int]bool),
		isFirstFetch:   true,
	}
}

// SetTray links the system tray manager to the application service.
func (s *AppService) SetTray(t *tray.TrayService) {
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

	client := gitlab.NewClient(conf.GitLabURL, conf.Token)
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

	client := gitlab.NewClient(conf.GitLabURL, conf.Token)

	// 1. Fetch User details to verify connection and get ID
	user, err := client.GetCurrentUser()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to GitLab: %w", err)
	}

	var wg sync.WaitGroup
	var todos []gitlab.Todo
	var mrs []gitlab.MergeRequest
	var todosErr, mrsErr error

	// Fetch Todos and MRs in parallel
	wg.Add(2)
	go func() {
		defer wg.Done()
		todos, todosErr = client.GetTodos()
	}()
	go func() {
		defer wg.Done()
		mrs, mrsErr = client.GetMergeRequests(user.ID)
	}()

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

	// 3. Fetch Pipelines for all resolved projects concurrently
	pipelines := make([]gitlab.PipelineWithJobs, 0)
	var pipeMu sync.Mutex
	var pipeWg sync.WaitGroup

	for projectPath := range dedupProjects {
		pipeWg.Add(1)
		go func(path string) {
			defer pipeWg.Done()
			pipe, err := client.GetPipelineWithJobs(path)
			if err == nil && pipe != nil {
				pipeMu.Lock()
				pipelines = append(pipelines, *pipe)
				pipeMu.Unlock()
			}
		}(projectPath)
	}

	// Wait for Todos, MRs, and Pipelines to complete
	wg.Wait()
	pipeWg.Wait()

	if todosErr != nil {
		return nil, fmt.Errorf("failed to fetch todos: %w", todosErr)
	}
	if mrsErr != nil {
		return nil, fmt.Errorf("failed to fetch merge requests: %w", mrsErr)
	}

	// 4. Process pipeline transitions & trigger system alerts
	passingCount := 0
	failingCount := 0
	runningCount := 0

	s.stateMu.Lock()
	for _, pwj := range pipelines {
		if pwj.Pipeline.ID == 0 {
			continue // No pipelines found for this project
		}

		status := pwj.Pipeline.Status
		path := pwj.ProjectPath
		name := pwj.ProjectName
		ref := pwj.Pipeline.Ref

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
		prevStatus, exists := s.pipelineStates[path]
		if exists && prevStatus != status {
			if status == "success" && conf.Notifications.Enabled && conf.Notifications.PipelineSuccess {
				s.trayService.Notify("Pipeline Passed", fmt.Sprintf("%s: pipeline passed on branch %s", name, ref))
			} else if status == "failed" && conf.Notifications.Enabled && conf.Notifications.PipelineFailed {
				s.trayService.Notify("Pipeline Failed", fmt.Sprintf("%s: pipeline failed on branch %s", name, ref))
			}
		}
		s.pipelineStates[path] = status
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
					s.trayService.Notify(title, fmt.Sprintf("%s: %s", todo.Project.PathWithNamespace, body))
				}
			}
		}
	}

	// 6. Process new Merge Requests transitions
	for _, mr := range mrs {
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
					s.trayService.Notify("Review Request", fmt.Sprintf("You are requested to review: %s", mr.Title))
				} else if isAssignee && conf.Notifications.MRAssigned {
					s.trayService.Notify("MR Assigned", fmt.Sprintf("Merge Request assigned to you: %s", mr.Title))
				}
			}
		}
	}

	s.isFirstFetch = false

	// Update system tray label
	if s.trayService != nil {
		s.trayService.UpdateTray(passingCount, failingCount, runningCount)
	}
	s.stateMu.Unlock()

	return &gitlab.TelemetryPayload{
		Todos:         todos,
		Pipelines:     pipelines,
		MergeRequests: mrs,
		Username:      user.Username,
		AvatarURL:     user.AvatarURL,
	}, nil
}

// MergeMergeRequest accept/merges the GitLab MR.
func (s *AppService) MergeMergeRequest(projectID int, mrIID int) error {
	conf, err := config.LoadConfig()
	if err != nil {
		return err
	}
	if conf.Token == "" {
		return fmt.Errorf("GitLab token not configured")
	}

	client := gitlab.NewClient(conf.GitLabURL, conf.Token)
	return client.MergeMergeRequest(projectID, mrIID)
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

	client := gitlab.NewClient(conf.GitLabURL, conf.Token)
	return client.CloseMergeRequest(projectID, mrIID)
}
