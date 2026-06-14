package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"
)

type cacheEntry struct {
	data      []byte
	etag      string
	fetchedAt time.Time
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

// ProxyConfig holds SOCKS5 proxy configuration.
type ProxyConfig struct {
	Enabled  bool
	Host     string
	Port     int
	User     string
	Password string
}

// NewClient initializes a new GitLab API client.
func NewClient(baseURL, token string, proxyConf *ProxyConfig) *Client {
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	// Create a token bucket rate limiter: 8 requests per second
	limiter := time.Tick(125 * time.Millisecond)

	transport := &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		MaxConnsPerHost:     5, // Limit concurrent connections to 5 to prevent server-side block/drop
		IdleConnTimeout:     90 * time.Second,
	}

	if proxyConf != nil && proxyConf.Enabled && proxyConf.Host != "" && proxyConf.Port > 0 {
		var auth *proxy.Auth
		if proxyConf.User != "" || proxyConf.Password != "" {
			auth = &proxy.Auth{
				User:     proxyConf.User,
				Password: proxyConf.Password,
			}
		}
		addr := fmt.Sprintf("%s:%d", proxyConf.Host, proxyConf.Port)
		dialer, err := proxy.SOCKS5("tcp", addr, auth, proxy.Direct)
		if err != nil {
			fmt.Printf("[Go Backend] Failed to configure SOCKS5 proxy: %v\n", err)
		} else {
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			}
		}
	}

	return &Client{
		BaseURL:    baseURL,
		Token:      token,
		HTTPClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		cache:      make(map[string]*cacheEntry),
		limiter:    limiter,
	}
}

func isTerminalStatus(status string) bool {
	switch status {
	case "success", "failed", "canceled", "skipped", "manual":
		return true
	default:
		return false
	}
}

// doRequest performs a rate-limited, cached HTTP request with a default 10s TTL.
func (c *Client) doRequest(apiPath string) ([]byte, bool, error) {
	return c.doRequestWithTTL(apiPath, 10*time.Second)
}

// doRequestWithTTL performs a rate-limited, cached HTTP request with a custom TTL.
func (c *Client) doRequestWithTTL(apiPath string, ttl time.Duration) ([]byte, bool, error) {
	fullURL := fmt.Sprintf("%s/api/v4/%s", c.BaseURL, strings.TrimPrefix(apiPath, "/"))

	// 1. Check if cached and within custom TTL to bypass network & rate limits
	c.cacheMu.Lock()
	entry, cached := c.cache[fullURL]
	if cached && time.Since(entry.fetchedAt) < ttl {
		c.cacheMu.Unlock()
		fmt.Printf("[Go Backend] doRequest Cache HIT (TTL): %s\n", apiPath)
		return entry.data, true, nil
	}
	c.cacheMu.Unlock()

	fmt.Printf("[Go Backend] doRequest path=%s (waiting for rate limit)\n", apiPath)
	// Wait for rate limit slot
	<-c.limiter
	fmt.Printf("[Go Backend] doRequest path=%s (got rate limit, starting HTTP request)\n", apiPath)

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, false, err
	}

	req.Header.Set("PRIVATE-TOKEN", c.Token)
	req.Header.Set("User-Agent", "Gittar-DevOps-Panel")

	// Apply ETag cache headers if available
	if cached && entry.etag != "" {
		req.Header.Set("If-None-Match", entry.etag)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotModified {
		if cached {
			// Update the fetchedAt timestamp so we don't request again for another TTL cycle
			c.cacheMu.Lock()
			entry.fetchedAt = time.Now()
			c.cacheMu.Unlock()
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

	// Cache the response and update fetchedAt
	c.cacheMu.Lock()
	c.cache[fullURL] = &cacheEntry{
		data:      bodyBytes,
		etag:      resp.Header.Get("ETag"),
		fetchedAt: time.Now(),
	}
	c.cacheMu.Unlock()

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

// GetSingleMergeRequest fetches detailed information for a single MR.
func (c *Client) GetSingleMergeRequest(projectID int, mrIID int, updatedAt time.Time) (*MergeRequest, error) {
	path := fmt.Sprintf("projects/%d/merge_requests/%d", projectID, mrIID)
	if !updatedAt.IsZero() {
		path = fmt.Sprintf("projects/%d/merge_requests/%d?cache_updated_at=%d", projectID, mrIID, updatedAt.Unix())
	}

	fullURL := fmt.Sprintf("%s/api/v4/%s", c.BaseURL, strings.TrimPrefix(path, "/"))

	// Inspect existing cache entry to dynamically determine TTL based on current pipeline status
	c.cacheMu.Lock()
	entry, cached := c.cache[fullURL]
	if cached {
		var cachedMR MergeRequest
		if err := json.Unmarshal(entry.data, &cachedMR); err == nil {
			ttl := 10 * time.Second
			if cachedMR.HeadPipeline != nil {
				if isTerminalStatus(cachedMR.HeadPipeline.Status) {
					ttl = 1 * time.Hour
				}
			} else {
				// MR has no pipeline. Caching for 10 mins is safe because any commit pushes change updatedAt.
				ttl = 10 * time.Minute
			}

			if time.Since(entry.fetchedAt) < ttl {
				c.cacheMu.Unlock()
				fmt.Printf("[Go Backend] GetSingleMergeRequest Cache HIT (Dynamic TTL): %s\n", path)
				return &cachedMR, nil
			}
		}
	}
	c.cacheMu.Unlock()

	// Cache miss or expired: fetch fresh data with 10s default TTL
	data, _, err := c.doRequestWithTTL(path, 10*time.Second)
	if err != nil {
		return nil, err
	}

	var mr MergeRequest
	if err := json.Unmarshal(data, &mr); err != nil {
		return nil, err
	}
	return &mr, nil
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

	// Fetch detailed MR information for each MR concurrently to populate head_pipeline if missing
	var wg sync.WaitGroup
	var mu sync.Mutex
	detailedMRs := make([]MergeRequest, len(result))

	for i, mr := range result {
		wg.Add(1)
		go func(idx int, m MergeRequest) {
			defer wg.Done()
			if m.HeadPipeline == nil {
				detailed, err := c.GetSingleMergeRequest(m.ProjectID, m.IID, m.UpdatedAt)
				if err == nil && detailed != nil {
					mu.Lock()
					detailedMRs[idx] = *detailed
					mu.Unlock()
					return
				}
			}
			mu.Lock()
			detailedMRs[idx] = m
			mu.Unlock()
		}(i, mr)
	}
	wg.Wait()

	return detailedMRs, nil
}

// GetProjectMergeRequests fetches open merge requests for a project.
func (c *Client) GetProjectMergeRequests(projectIDOrPath string) ([]MergeRequest, error) {
	escapedPath := url.PathEscape(projectIDOrPath)
	data, _, err := c.doRequest(fmt.Sprintf("projects/%s/merge_requests?state=opened&per_page=50", escapedPath))
	if err != nil {
		return nil, err
	}

	var mrs []MergeRequest
	if err := json.Unmarshal(data, &mrs); err != nil {
		return nil, err
	}

	// Fetch detailed MR information for each MR concurrently to populate head_pipeline if missing
	var wg sync.WaitGroup
	var mu sync.Mutex
	detailedMRs := make([]MergeRequest, len(mrs))

	for i, mr := range mrs {
		wg.Add(1)
		go func(idx int, m MergeRequest) {
			defer wg.Done()
			if m.HeadPipeline == nil {
				detailed, err := c.GetSingleMergeRequest(m.ProjectID, m.IID, m.UpdatedAt)
				if err == nil && detailed != nil {
					mu.Lock()
					detailedMRs[idx] = *detailed
					mu.Unlock()
					return
				}
			}
			mu.Lock()
			detailedMRs[idx] = m
			mu.Unlock()
		}(i, mr)
	}
	wg.Wait()


	return detailedMRs, nil
}

// GetPipelinesWithJobs fetches up to limit pipelines for a project, along with their individual jobs.
func (c *Client) GetPipelinesWithJobs(projectIDOrPath string, limit int) ([]PipelineWithJobs, error) {
	escapedPath := url.PathEscape(projectIDOrPath)

	// 1. Get project details to know name/path
	proj, err := c.GetProjectDetails(projectIDOrPath)
	if err != nil {
		return nil, err
	}

	// 2. Get latest pipelines
	pipelinesData, _, err := c.doRequest(fmt.Sprintf("projects/%s/pipelines?per_page=%d", escapedPath, limit))
	if err != nil {
		return nil, err
	}

	var pipelines []Pipeline
	if err := json.Unmarshal(pipelinesData, &pipelines); err != nil {
		return nil, err
	}

	if len(pipelines) == 0 {
		return []PipelineWithJobs{
			{
				ProjectName: proj.Name,
				ProjectPath: proj.PathWithNamespace,
			},
		}, nil
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]PipelineWithJobs, len(pipelines))

	for i, pipe := range pipelines {
		wg.Add(1)
		go func(idx int, p Pipeline) {
			defer wg.Done()

			// Fetch full pipeline details to get duration/user info if available
			detailedPipeline := p
			ttl := 10 * time.Second
			if isTerminalStatus(p.Status) {
				ttl = 1 * time.Hour
			}

			fullPipeData, _, err := c.doRequestWithTTL(fmt.Sprintf("projects/%s/pipelines/%d", escapedPath, p.ID), ttl)
			if err == nil {
				var detailed Pipeline
				if err2 := json.Unmarshal(fullPipeData, &detailed); err2 == nil {
					detailedPipeline = detailed
				}
			}

			// Fetch pipeline jobs
			var jobs []Job
			jobsData, _, err3 := c.doRequestWithTTL(fmt.Sprintf("projects/%s/pipelines/%d/jobs?per_page=50", escapedPath, p.ID), ttl)
			if err3 == nil {
				_ = json.Unmarshal(jobsData, &jobs)
			}

			mu.Lock()
			results[idx] = PipelineWithJobs{
				Pipeline:    detailedPipeline,
				Jobs:        jobs,
				ProjectName: proj.Name,
				ProjectPath: proj.PathWithNamespace,
			}
			mu.Unlock()
		}(i, pipe)
	}
	wg.Wait()

	return results, nil
}

// GetJobLogSnippet fetches the final 10 lines of a job's build log on failure.
func (c *Client) GetJobLogSnippet(projectIDOrPath string, jobID int) (string, error) {
	fmt.Printf("[Go Backend] GetJobLogSnippet project=%s, jobID=%d (fetching trace)\n", projectIDOrPath, jobID)
	escapedPath := url.PathEscape(projectIDOrPath)
	data, _, err := c.doRequest(fmt.Sprintf("projects/%s/jobs/%d/trace", escapedPath, jobID))
	if err != nil {
		fmt.Printf("[Go Backend] GetJobLogSnippet project=%s, jobID=%d error: %v\n", projectIDOrPath, jobID, err)
		return "", err
	}

	logStr := string(data)
	lines := strings.Split(logStr, "\n")
	if len(lines) > 20 {
		lines = lines[len(lines)-20:]
	}
	fmt.Printf("[Go Backend] GetJobLogSnippet project=%s, jobID=%d success: fetched %d bytes, tail lines=%d\n", projectIDOrPath, jobID, len(data), len(lines))
	return strings.Join(lines, "\n"), nil
}

// GetGroupProjects fetches all projects under a group, including nested subgroups, handling pagination.
func (c *Client) GetGroupProjects(groupIDOrPath string) ([]ProjectRef, error) {
	escapedPath := url.PathEscape(groupIDOrPath)
	var allProjects []ProjectRef
	page := 1
	for {
		path := fmt.Sprintf("groups/%s/projects?simple=true&include_subgroups=true&per_page=100&page=%d", escapedPath, page)
		data, _, err := c.doRequest(path)
		if err != nil {
			return nil, err
		}

		var projects []ProjectRef
		if err := json.Unmarshal(data, &projects); err != nil {
			return nil, err
		}

		if len(projects) == 0 {
			break
		}

		allProjects = append(allProjects, projects...)
		if len(projects) < 100 {
			break
		}
		page++
	}
	return allProjects, nil
}

// doWriteRequest executes a write operation (POST/PUT/DELETE) on the GitLab API.
func (c *Client) doWriteRequest(method, apiPath string, body interface{}) ([]byte, error) {
	// Wait for rate limit slot
	<-c.limiter

	fullURL := fmt.Sprintf("%s/api/v4/%s", c.BaseURL, strings.TrimPrefix(apiPath, "/"))

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("PRIVATE-TOKEN", c.Token)
	req.Header.Set("User-Agent", "Gittar-DevOps-Panel")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gitlab api error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	return io.ReadAll(resp.Body)
}

// MergeMergeRequest accepts/merges the MR.
func (c *Client) MergeMergeRequest(projectID int, mrIID int) error {
	path := fmt.Sprintf("projects/%d/merge_requests/%d/merge", projectID, mrIID)
	_, err := c.doWriteRequest("PUT", path, nil)
	return err
}

// CloseMergeRequest updates the MR state to closed.
func (c *Client) CloseMergeRequest(projectID int, mrIID int) error {
	path := fmt.Sprintf("projects/%d/merge_requests/%d", projectID, mrIID)
	body := map[string]string{"state_event": "close"}
	_, err := c.doWriteRequest("PUT", path, body)
	return err
}

// MarkTodoAsDone marks a pending todo as done.
func (c *Client) MarkTodoAsDone(todoID int) error {
	path := fmt.Sprintf("todos/%d/mark_as_done", todoID)
	_, err := c.doWriteRequest("POST", path, nil)
	return err
}

// RetryPipeline retries a failed pipeline.
func (c *Client) RetryPipeline(projectPath string, pipelineID int) error {
	escapedPath := url.PathEscape(projectPath)
	path := fmt.Sprintf("projects/%s/pipelines/%d/retry", escapedPath, pipelineID)
	_, err := c.doWriteRequest("POST", path, nil)
	return err
}

// CancelPipeline cancels a running pipeline.
func (c *Client) CancelPipeline(projectPath string, pipelineID int) error {
	escapedPath := url.PathEscape(projectPath)
	path := fmt.Sprintf("projects/%s/pipelines/%d/cancel", escapedPath, pipelineID)
	_, err := c.doWriteRequest("POST", path, nil)
	return err
}

// GetIssues fetches issues authored by or assigned to the user.
func (c *Client) GetIssues(userID int) ([]Issue, error) {
	// Fetch assigned issues
	assignedData, _, err := c.doRequest("issues?state=opened&scope=assigned_to_me&per_page=50")
	if err != nil {
		return nil, err
	}

	var assigned []Issue
	if err := json.Unmarshal(assignedData, &assigned); err != nil {
		return nil, err
	}

	// Fetch authored issues
	authoredData, _, err := c.doRequest("issues?state=opened&scope=created_by_me&per_page=50")
	if err != nil {
		return nil, err
	}

	var authored []Issue
	if err := json.Unmarshal(authoredData, &authored); err != nil {
		return nil, err
	}

	// Merge & Deduplicate
	issueMap := make(map[int]Issue)
	for _, issue := range assigned {
		issueMap[issue.ID] = issue
	}
	for _, issue := range authored {
		issueMap[issue.ID] = issue
	}

	result := make([]Issue, 0, len(issueMap))
	for _, issue := range issueMap {
		result = append(result, issue)
	}

	return result, nil
}

// GetProjectIssues fetches open issues for a project.
func (c *Client) GetProjectIssues(projectIDOrPath string) ([]Issue, error) {
	escapedPath := url.PathEscape(projectIDOrPath)
	data, _, err := c.doRequest(fmt.Sprintf("projects/%s/issues?state=opened&per_page=50", escapedPath))
	if err != nil {
		return nil, err
	}

	var issues []Issue
	if err := json.Unmarshal(data, &issues); err != nil {
		return nil, err
	}

	return issues, nil
}

// CloseIssue updates the issue state to closed.
func (c *Client) CloseIssue(projectID int, issueIID int) error {
	path := fmt.Sprintf("projects/%d/issues/%d", projectID, issueIID)
	body := map[string]string{"state_event": "close"}
	_, err := c.doWriteRequest("PUT", path, body)
	return err
}

// ClearCache flushes all cached HTTP responses.
func (c *Client) ClearCache() {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	c.cache = make(map[string]*cacheEntry)
}

// GetMergeRequestCommits fetches up to 100 commits in a merge request.
func (c *Client) GetMergeRequestCommits(projectID int, mrIID int) ([]Commit, error) {
	path := fmt.Sprintf("projects/%d/merge_requests/%d/commits?per_page=100", projectID, mrIID)
	data, _, err := c.doRequest(path)
	if err != nil {
		return nil, err
	}

	var commits []Commit
	if err := json.Unmarshal(data, &commits); err != nil {
		return nil, err
	}
	return commits, nil
}

// GetMergeRequestNotes fetches comments/activity for a merge request.
func (c *Client) GetMergeRequestNotes(projectID int, mrIID int) ([]Note, error) {
	path := fmt.Sprintf("projects/%d/merge_requests/%d/notes?sort=asc&per_page=100", projectID, mrIID)
	data, _, err := c.doRequest(path)
	if err != nil {
		return nil, err
	}

	var notes []Note
	if err := json.Unmarshal(data, &notes); err != nil {
		return nil, err
	}
	return notes, nil
}

// CreateMergeRequestNote adds a new comment to a merge request.
func (c *Client) CreateMergeRequestNote(projectID int, mrIID int, body string) (*Note, error) {
	path := fmt.Sprintf("projects/%d/merge_requests/%d/notes", projectID, mrIID)
	reqBody := map[string]string{"body": body}
	data, err := c.doWriteRequest("POST", path, reqBody)
	if err != nil {
		return nil, err
	}

	var note Note
	if err := json.Unmarshal(data, &note); err != nil {
		return nil, err
	}
	return &note, nil
}

