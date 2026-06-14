package gitlab

import "time"

// User represents a GitLab user.
type User struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	State     string `json:"state"`
	AvatarURL string `json:"avatar_url"`
	WebURL    string `json:"web_url"`
}

// ProjectRef represents a simplified project reference.
type ProjectRef struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	PathWithNamespace string `json:"path_with_namespace"`
	WebURL            string `json:"web_url"`
}

// Todo represents a GitLab Todo item.
type Todo struct {
	ID         int        `json:"id"`
	Project    ProjectRef `json:"project"`
	Author     User       `json:"author"`
	ActionName string     `json:"action_name"`
	TargetType string     `json:"target_type"`
	TargetURL  string     `json:"target_url"`
	Body       string     `json:"body"`
	State      string     `json:"state"`
	CreatedAt  time.Time  `json:"created_at"`
}

// Pipeline represents a GitLab Pipeline.
type Pipeline struct {
	ID        int       `json:"id"`
	IID       int       `json:"iid"`
	ProjectID int       `json:"project_id"`
	Status    string    `json:"status"` // running, pending, success, failed, canceled, skipped, manual
	Ref       string    `json:"ref"`
	SHA       string    `json:"sha"`
	WebURL    string    `json:"web_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Duration  int       `json:"duration"` // in seconds
	User      User      `json:"user"`
}

// Job represents a GitLab Job.
type Job struct {
	ID         int        `json:"id"`
	Status     string     `json:"status"`
	Stage      string     `json:"stage"`
	Name       string     `json:"name"`
	Ref        string     `json:"ref"`
	WebURL     string     `json:"web_url"`
	Duration   float64    `json:"duration"` // in seconds
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// HeadPipeline represents the pipeline for the head commit of a Merge Request.
type HeadPipeline struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
	WebURL string `json:"web_url"`
}

// MergeRequest represents a GitLab Merge Request.
type MergeRequest struct {
	ID             int           `json:"id"`
	IID            int           `json:"iid"`
	ProjectID      int           `json:"project_id"`
	Title          string        `json:"title"`
	Description    string        `json:"description"`
	State          string        `json:"state"`
	TargetBranch   string        `json:"target_branch"`
	SourceBranch   string        `json:"source_branch"`
	Author         User          `json:"author"`
	Assignees      []User        `json:"assignees"`
	Reviewers      []User        `json:"reviewers"`
	WebURL         string        `json:"web_url"`
	UserNotesCount int           `json:"user_notes_count"`
	Upvotes        int           `json:"upvotes"`
	Downvotes      int           `json:"downvotes"`
	WorkInProgress bool          `json:"work_in_progress"`
	Draft          bool          `json:"draft"`
	Labels         []string      `json:"labels"`
	HeadPipeline   *HeadPipeline `json:"head_pipeline,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
	MergedAt       *time.Time    `json:"merged_at,omitempty"`
	ClosedAt       *time.Time    `json:"closed_at,omitempty"`
}

// Issue represents a GitLab Issue.
type Issue struct {
	ID             int        `json:"id"`
	IID            int        `json:"iid"`
	ProjectID      int        `json:"project_id"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	State          string     `json:"state"`
	Author         User       `json:"author"`
	Assignees      []User     `json:"assignees"`
	Labels         []string   `json:"labels"`
	WebURL         string     `json:"web_url"`
	UserNotesCount int        `json:"user_notes_count"`
	Upvotes        int        `json:"upvotes"`
	Downvotes      int        `json:"downvotes"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ClosedAt       *time.Time `json:"closed_at,omitempty"`
}

// PipelineWithJobs bundles a pipeline with its current jobs and project context.
type PipelineWithJobs struct {
	Pipeline    Pipeline `json:"pipeline"`
	Jobs        []Job    `json:"jobs"`
	ProjectName string   `json:"projectName"`
	ProjectPath string   `json:"projectPath"`
}

// TelemetryPayload is the unified status update pushed to the frontend.
type TelemetryPayload struct {
	Todos         []Todo             `json:"todos"`
	Pipelines     []PipelineWithJobs `json:"pipelines"`
	MergeRequests []MergeRequest     `json:"mergeRequests"`
	Issues        []Issue            `json:"issues"`
	Username      string             `json:"username"`
	AvatarURL     string             `json:"avatarUrl"`
	Error         string             `json:"error,omitempty"`
}
