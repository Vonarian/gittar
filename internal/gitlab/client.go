package gitlab

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type cacheEntry struct {
	data []byte
	etag string
}

// Client is a rate-limited, ETag-cached GitLab client.
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
	cache      map[string]*cacheEntry
	cacheMu    sync.Mutex
	limiter    <-chan time.Time
}

// NewClient initializes a new GitLab API client.
func NewClient(baseURL, token string) *Client {
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	// Create a token bucket rate limiter: 8 requests per second
	limiter := time.Tick(125 * time.Millisecond)

	return &Client{
		BaseURL:    baseURL,
		Token:      token,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		cache:      make(map[string]*cacheEntry),
		limiter:    limiter,
	}
}

// doRequest performs a rate-limited, cached HTTP request.
// It returns the response body data, a boolean indicating if it was from cache, and an error.
func (c *Client) doRequest(apiPath string) ([]byte, bool, error) {
	// Wait for rate limit slot
	<-c.limiter

	fullURL := fmt.Sprintf("%s/api/v4/%s", c.BaseURL, strings.TrimPrefix(apiPath, "/"))

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, false, err
	}

	req.Header.Set("PRIVATE-TOKEN", c.Token)
	req.Header.Set("User-Agent", "Gittar-DevOps-Panel")

	// Apply ETag cache headers
	c.cacheMu.Lock()
	entry, cached := c.cache[fullURL]
	c.cacheMu.Unlock()

	if cached && entry.etag != "" {
		req.Header.Set("If-None-Match", entry.etag)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		if cached {
			return entry.data, true, nil
		}
		return nil, false, fmt.Errorf("server returned 304 but cache entry was missing")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, false, fmt.Errorf("gitlab api error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}

	// Update cache with new ETag
	etag := resp.Header.Get("ETag")
	if etag != "" {
		c.cacheMu.Lock()
		c.cache[fullURL] = &cacheEntry{
			data: bodyBytes,
			etag: etag,
		}
		c.cacheMu.Unlock()
	}

	return bodyBytes, false, nil
}

// GetCurrentUser returns details of the currently authenticated user.
func (c *Client) GetCurrentUser() (*User, error) {
	data, _, err := c.doRequest("user")
	if err != nil {
		return nil, err
	}

	var user User
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// GetTodos fetches the active todo items for the user.
func (c *Client) GetTodos() ([]Todo, error) {
	data, _, err := c.doRequest("todos?state=pending&per_page=50")
	if err != nil {
		return nil, err
	}

	var todos []Todo
	if err := json.Unmarshal(data, &todos); err != nil {
		return nil, err
	}
	return todos, nil
}

// GetProjectDetails fetches details of a specific project.
func (c *Client) GetProjectDetails(projectIDOrPath string) (*ProjectRef, error) {
	escapedPath := url.PathEscape(projectIDOrPath)
	data, _, err := c.doRequest(fmt.Sprintf("projects/%s", escapedPath))
	if err != nil {
		return nil, err
	}

	var p ProjectRef
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// GetPipelineWithJobs fetches the latest pipeline for a project, along with its individual jobs.
func (c *Client) GetPipelineWithJobs(projectIDOrPath string) (*PipelineWithJobs, error) {
	escapedPath := url.PathEscape(projectIDOrPath)
	
	// 1. Get project details to know name/path
	proj, err := c.GetProjectDetails(projectIDOrPath)
	if err != nil {
		return nil, err
	}

	// 2. Get latest pipeline
	pipelinesData, _, err := c.doRequest(fmt.Sprintf("projects/%s/pipelines?per_page=1", escapedPath))
	if err != nil {
		return nil, err
	}

	var pipelines []Pipeline
	if err := json.Unmarshal(pipelinesData, &pipelines); err != nil {
		return nil, err
	}

	if len(pipelines) == 0 {
		return &PipelineWithJobs{
			ProjectName: proj.Name,
			ProjectPath: proj.PathWithNamespace,
		}, nil
	}

	latestPipeline := pipelines[0]

	// 3. Fetch full pipeline details to get duration/user info if available
	fullPipeData, _, err := c.doRequest(fmt.Sprintf("projects/%s/pipelines/%d", escapedPath, latestPipeline.ID))
	if err == nil {
		var detailedPipeline Pipeline
		if err := json.Unmarshal(fullPipeData, &detailedPipeline); err == nil {
			latestPipeline = detailedPipeline
		}
	}

	// 4. Fetch pipeline jobs
	jobsData, _, err := c.doRequest(fmt.Sprintf("projects/%s/pipelines/%d/jobs?per_page=50", escapedPath, latestPipeline.ID))
	if err != nil {
		return &PipelineWithJobs{
			Pipeline:    latestPipeline,
			ProjectName: proj.Name,
			ProjectPath: proj.PathWithNamespace,
		}, nil
	}

	var jobs []Job
	if err := json.Unmarshal(jobsData, &jobs); err != nil {
		return &PipelineWithJobs{
			Pipeline:    latestPipeline,
			ProjectName: proj.Name,
			ProjectPath: proj.PathWithNamespace,
		}, nil
	}

	return &PipelineWithJobs{
		Pipeline:    latestPipeline,
		Jobs:        jobs,
		ProjectName: proj.Name,
		ProjectPath: proj.PathWithNamespace,
	}, nil
}

// GetMergeRequests fetches merge requests authored by or assigned to the user.
func (c *Client) GetMergeRequests(userID int) ([]MergeRequest, error) {
	// Fetch assigned MRs
	assignedData, _, err := c.doRequest("merge_requests?state=opened&scope=assigned_to_me&per_page=30")
	if err != nil {
		return nil, err
	}

	var assigned []MergeRequest
	if err := json.Unmarshal(assignedData, &assigned); err != nil {
		return nil, err
	}

	// Fetch authored MRs
	authoredData, _, err := c.doRequest("merge_requests?state=opened&scope=created_by_me&per_page=30")
	if err != nil {
		return nil, err
	}

	var authored []MergeRequest
	if err := json.Unmarshal(authoredData, &authored); err != nil {
		return nil, err
	}

	// Fetch review requests MRs (where current user is a reviewer)
	reviewerPath := fmt.Sprintf("merge_requests?state=opened&reviewer_id=%d&per_page=30", userID)
	reviewerData, _, err := c.doRequest(reviewerPath)
	var reviewerRequests []MergeRequest
	if err == nil {
		_ = json.Unmarshal(reviewerData, &reviewerRequests)
	}

	// Merge & Deduplicate
	mrMap := make(map[int]MergeRequest)
	for _, mr := range assigned {
		mrMap[mr.ID] = mr
	}
	for _, mr := range authored {
		mrMap[mr.ID] = mr
	}
	for _, mr := range reviewerRequests {
		mrMap[mr.ID] = mr
	}

	result := make([]MergeRequest, 0, len(mrMap))
	for _, mr := range mrMap {
		result = append(result, mr)
	}

	return result, nil
}

// GetJobLogSnippet fetches the final 10 lines of a job's build log on failure.
func (c *Client) GetJobLogSnippet(projectIDOrPath string, jobID int) (string, error) {
	escapedPath := url.PathEscape(projectIDOrPath)
	data, _, err := c.doRequest(fmt.Sprintf("projects/%s/jobs/%d/trace", escapedPath, jobID))
	if err != nil {
		return "", err
	}

	logStr := string(data)
	lines := strings.Split(logStr, "\n")
	if len(lines) > 20 {
		lines = lines[len(lines)-20:]
	}
	return strings.Join(lines, "\n"), nil
}

// GetGroupProjects fetches projects under a group.
func (c *Client) GetGroupProjects(groupIDOrPath string) ([]ProjectRef, error) {
	escapedPath := url.PathEscape(groupIDOrPath)
	data, _, err := c.doRequest(fmt.Sprintf("groups/%s/projects?simple=true&per_page=30", escapedPath))
	if err != nil {
		return nil, err
	}

	var projects []ProjectRef
	if err := json.Unmarshal(data, &projects); err != nil {
		return nil, err
	}
	return projects, nil
}
