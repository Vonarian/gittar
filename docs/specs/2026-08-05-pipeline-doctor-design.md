# Design Spec: AI CI/CD Pipeline Failure Root-Cause Analyzer ("Pipeline Doctor")

Date: 2026-08-05

## 1. Overview
Gittar is a high-performance visual control panel and menu bar anchor for GitLab organizations. This feature adds real-time AI-powered root-cause analysis (RCA) and fix recommendations for failed CI/CD pipelines and job traces.

## 2. Architecture & Components

```
+-----------------------------------------------------------------------+
| Svelte 5 Frontend (PipelinesPanel / InspectorPanel)                  |
|  - "Analyze Failure" button on failed jobs                            |
|  - Renders Root Cause, Error Snippet, and Suggested Fix               |
+-----------------------------------+-----------------------------------+
                                    | Wails Binding Call
                                    v
+-----------------------------------------------------------------------+
| AppService (internal/service/app.go)                                  |
|  - AnalyzePipelineFailure(projectPath, jobID, force)                  |
|  - Fetches trace log via GitLab API                                   |
|  - Cleans ANSI escape codes & isolates error tail                     |
|  - Applies AICostPreset limits                                        |
|  - Caches RCA results in memory                                       |
+-----------------------------------+-----------------------------------+
                                    |
                                    v
+-----------------------------------------------------------------------+
| OpenAIClient (internal/ai/openai.go)                                  |
|  - Formats structured prompt for OpenAI / OpenRouter / Ollama        |
|  - Parses markdown response containing Cause, Snippet, Fix            |
+-----------------------------------------------------------------------+
```

## 3. Detailed Data Flow

1. User clicks **"Analyze Failure"** on a failed job pill in `PipelinesPanel` or within `InspectorPanel`.
2. Frontend calls `AppService.AnalyzePipelineFailure(projectPath, jobID, force)`.
3. Backend checks `pipelineRCACache[fmt.Sprintf("%s:%d", projectPath, jobID)]`. If cached and `!force`, return cached result immediately.
4. Backend fetches job log snippet using `client.GetJobTrace(projectPath, jobID)`.
5. ANSI codes are stripped and the log is truncated according to `conf.AICostPreset`.
6. Prompt is constructed directing the LLM to output a structured markdown response with:
   - **Root Cause**: Concise summary of the error context.
   - **Error Snippet**: The specific lines of code or log snippet causing the failure.
   - **Suggested Fix**: Recommended code patch or fix action.
7. Backend caches and returns the markdown result to the Svelte frontend.

## 4. UI/UX Specifications
- **Theme**: High-density macOS dark-mode / translucent styling matching Gittar design system.
- **Components**:
  - `InspectorPanel.svelte`: AI card rendered above raw log view with tab toggle between "AI Diagnosis" and "Raw Log Trace".
  - `PipelinesPanel.svelte`: Direct button action on failed jobs.
- **Copy Action**: 1-click button to copy suggested fix to clipboard using Wails clipboard API.

## 5. Performance & Resource Constraints
- **Caching**: 10-minute TTL memory cache for job RCA results.
- **Rate-Limiting**: Bypasses network call on cached hits; uses configured HTTP Keep-Alive client.
- **Token Limits**:
  - Low preset: ~1,500 chars log window.
  - Medium preset: ~3,500 chars log window.
  - High preset: ~7,000 chars log window.
