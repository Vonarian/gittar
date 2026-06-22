package ai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIClient_GenerateSummary_Success(t *testing.T) {
	mockResponse := ChatCompletionResponse{
		Choices: []Choice{
			{
				Message: Message{
					Role:    "assistant",
					Content: "Test OpenAI MR summary",
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("expected Bearer token header, got %s", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := NewOpenAIClient("test-api-key", "gpt-4o-mini", server.URL)

	summary, err := client.GenerateSummary("summarize this code change")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary != "Test OpenAI MR summary" {
		t.Errorf("expected summary 'Test OpenAI MR summary', got %q", summary)
	}
}

func TestOpenAIClient_GenerateSummary_NoKey(t *testing.T) {
	mockResponse := ChatCompletionResponse{
		Choices: []Choice{
			{
				Message: Message{
					Role:    "assistant",
					Content: "Test local model summary",
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("expected no Authorization header, got %s", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := NewOpenAIClient("", "llama3", server.URL)

	summary, err := client.GenerateSummary("summarize this code change")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary != "Test local model summary" {
		t.Errorf("expected summary 'Test local model summary', got %q", summary)
	}
}

func TestOpenAIClient_GenerateSummary_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid request"))
	}))
	defer server.Close()

	client := NewOpenAIClient("test-api-key", "gpt-4o-mini", server.URL)

	_, err := client.GenerateSummary("summarize this")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
