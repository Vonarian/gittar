# Agent Instructions & Guidelines

Gittar is a high-performance visual control panel and menu bar anchor for GitLab organizations, providing unified real-time visibility and control over merge requests, pipelines, and todos with zero local git operations.

---

## 🔀 GitFlow & Branching Model
- **Strict GitFlow:** All development must occur in dedicated feature branches (`feature/your-feature-name`) branched from `develop`.
- **Merge Process:** Features must be merged into `develop` using a non-fast-forward merge commit (`git merge --no-ff`) to preserve history. Releases are merged from `develop` into `main` using `--no-ff`.
- **Conventional Commits:** Write clear, conventional commit messages (e.g., `fix: ...`, `feat: ...`, `docs: ...`) with a descriptive body explaining the *why* of the change.
- **Cleanup:** Delete local and remote feature branches immediately after they are successfully merged and pushed.

## 🧪 Testing & Quality Assurance (TDD)
- **Test-Driven Development (TDD):** Write unit tests concurrently with or prior to implementing business logic.
- **Go Backend Testing:** Ensure all internal services and GitLab API integration logic have corresponding unit tests. Run tests via:
  ```bash
  go test -v ./...
  ```
- **Go Linting:** Ensure all Go source files adhere to code quality standards with no warnings or errors. Running the following command is crucial for quality assurance before making any commits:
  ```bash
  golangci-lint-v2 run ./...
  ```
- **Frontend Type Safety:** Ensure TypeScript and Svelte components compile cleanly without errors. Validate using:
  ```bash
  npm run check
  ```

## ⚡ Performance & Resource Optimization
- **Non-Overlapping Polling:** Never use `setInterval` for background telemetry fetches. Always use a self-scheduling `setTimeout` combined with a boolean concurrency lock (e.g. `isFetching`) to prevent requests from piling up.
- **Caching & Rate Limiting:** Optimize API calls to stay within rate limits (e.g. 8 reqs/sec). Implement time-based memory caching (e.g. 10s TTL) for GET requests to bypass the network and rate limit queue.
- **Connection Pooling:** Cache and reuse the `gitlab.Client` instance inside `AppService` to leverage HTTP Keep-Alive connection pooling. Keep `MaxConnsPerHost` set to a safe limit (e.g. 5) to prevent socket exhaustion.
- **Non-Blocking Mounts:** Keep frontend mounting (`onMount`) completely non-blocking (avoid `await` on heavy fetches) to guarantee instant UI rendering.

## 🎨 UI/UX & Design Aesthetics
- **High-Density Layouts:** Group pipeline stages visually and sort collections alphabetically to prevent items from shifting positions on updates.
- **Keyed Svelte Loops:** Always key `{#each}` blocks on unique global IDs (like `mr.id` instead of project-scoped `mr.iid`) to enable fast Svelte DOM diffing and prevent lag.
- **Interactive Controls:** Prompt users with explicit confirmation dialogs before executing destructive actions (such as Merging or Closing MRs).
- **macOS Visual Integration:** Support translucent dark-mode panels, hidden titlebars with traffic light spacing, and standard Dock squircle app icon layout.
