# 🎸 Gittar

An API-driven, high-performance visual control panel and menu bar anchor for GitLab organizations. Built with **Go**, **Wails v3**, **Svelte 5**, and **Tailwind CSS v4**.

Gittar acts as a lightweight command center, utilizing GitLab's REST and GraphQL APIs to provide unified visibility and real-time control over merge requests, pipeline statuses, failed job logs, and incoming todos—with **zero local git operations**.

---

## ✨ Features

- **🛡️ Telemetry Sync Error Debouncing:** Tolerates transient network fluctuations or temporary timeouts by debouncing offline warnings until 3 consecutive sync cycles fail, gracefully displaying cached telemetry payload.
- **🔍 Comprehensive Section Filtering:** Dropdown filters for **Group**, **Project**, and **User** across all sections, alongside global text search.
- **📥 Inbox Feed (Todos):** Real-time prioritization of items needing action (approvals, reviews, assignments, @mentions).
- **🚀 Pipelines Matrix:** Compact compile-state grid with visual job nodes and direct failure log tail inspections.
- **🎛️ MR Gatekeeper:** Combined dashboard of all open merge requests assigned to, reviewed by, or authored by you, plus an **All** tab showing the full aggregate.
- **🔔 Fine-Tuned Notifications:** Complete settings checklists enabling/disabling desktop notifications for pipeline passes, failures, assignments, mentions, and issue updates.
- **🍏 Native macOS Visuals:** Vibrant translucency styling, hidden-inset traffic light window control spacers, and custom titlebar double-click maximization.

---

## 🛠️ Tech Stack

- **Backend:** Go 1.24+, Wails v3 (Desktop framework)
- **Frontend:** Svelte 5, TypeScript, Tailwind CSS v4, Vite
- **Security:** Zero local filesystem scanning, PAT token storage encrypted locally

---

## 📦 Getting Started

### Prerequisites

1. Install Go 1.24+
2. Install Node.js (v18+) and npm
3. Install Wails v3 CLI:
   ```bash
   go install github.com/wailsapp/wails/v3/cmd/wails3@latest
   ```

### Running in Development

To start Gittar in interactive live-reload dev mode:
```bash
wails3 dev
```
This runs the Go backend, launches Vite for the Svelte frontend, and opens a hot-reloaded development window.

### Compiling Production App

To build and package Gittar into a native macOS `.app` bundle:
```bash
wails3 package
```
The compiled, ad-hoc signed application bundle will be saved at `bin/gittar.app`.

---

## ⚙️ Configuration

Gittar saves configuration securely under your home directory at `~/.config/gittar/config.json`. 

```json
{
  "gitlabUrl": "https://gitlab.com",
  "token": "glpat-YOUR_PERSONAL_ACCESS_TOKEN",
  "monitoredGroups": [
    "your-org-group"
  ],
  "monitoredProjects": [
    "your-org-group/subgroup/project-name"
  ],
  "pollIntervalSec": 30,
  "notifications": {
    "enabled": true,
    "pipelineSuccess": true,
    "pipelineFailed": true,
    "mrAssigned": true,
    "mrReviewRequest": true,
    "todoMention": true,
    "todoAssignment": true,
    "todoIssue": true,
    "todoGeneric": true
  }
}
```

---

## 📝 License

Distributed under the MIT License. See `LICENSE` for more information.
