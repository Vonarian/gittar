package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Message represents a chat completion message role and content.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest is the OpenAI-compatible request payload format.
type ChatCompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// Choice represents a completion choice returned by the API.
type Choice struct {
	Message Message `json:"message"`
}

// ChatCompletionResponse is the response payload format.
type ChatCompletionResponse struct {
	Choices []Choice `json:"choices"`
}

// OpenAIClient makes direct HTTP API requests to any OpenAI-compatible API endpoint.
type OpenAIClient struct {
	APIKey  string
	Model   string
	BaseURL string
	Client  *http.Client
}

// NewOpenAIClient instantiates a new OpenAI-Compatible client.
func NewOpenAIClient(apiKey, model, baseURL string) *OpenAIClient {
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &OpenAIClient{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: baseURL,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GenerateSummary runs a chat completion call.
func (c *OpenAIClient) GenerateSummary(prompt string) (string, error) {
	if c.BaseURL == "" {
		return "", fmt.Errorf("AI base URL is not configured")
	}

	apiURL := fmt.Sprintf("%s/chat/completions", c.BaseURL)

	reqPayload := ChatCompletionRequest{
		Model: c.Model,
		Messages: []Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	jsonData, err := json.Marshal(reqPayload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request payload: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	}

	// Add special headers required by OpenRouter if calling OpenRouter
	if strings.Contains(c.BaseURL, "openrouter.ai") {
		req.Header.Set("HTTP-Referer", "https://github.com/vonarian/gittar")
		req.Header.Set("X-Title", "Gittar")
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http call to AI API failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("AI API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var respPayload ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&respPayload); err != nil {
		return "", fmt.Errorf("failed to decode response payload: %w", err)
	}

	if len(respPayload.Choices) == 0 {
		return "", fmt.Errorf("AI API returned no completion choices")
	}

	return respPayload.Choices[0].Message.Content, nil
}
