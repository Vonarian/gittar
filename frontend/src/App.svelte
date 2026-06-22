<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Window } from "@wailsio/runtime";
  import Sidebar from "./lib/components/Sidebar.svelte";
  import SetupPanel from "./lib/components/SetupPanel.svelte";
  import TodosPanel from "./lib/components/TodosPanel.svelte";
  import PipelinesPanel from "./lib/components/PipelinesPanel.svelte";
  import MRsPanel from "./lib/components/MRsPanel.svelte";
  import IssuesPanel from "./lib/components/IssuesPanel.svelte";
  import InspectorPanel from "./lib/components/InspectorPanel.svelte";
  import MRInspectorPanel from "./lib/components/MRInspectorPanel.svelte";

  import { FetchTelemetry, GetConfig, SaveConfig, GetCachedTelemetry } from "../bindings/gittar/internal/service/appservice";
  import type { TelemetryPayload, MergeRequest } from "../bindings/gittar/internal/gitlab/models";

  // Reactive state using Svelte 5 Runes
  let currentTab = $state("todos");
  let isWindows = $state(false);
  let isMaximised = $state(false);
  let isConfigured = $state(true);
  let isLoading = $state(false);
  let pollIntervalSec = $state(30);
  let ignoreFailedPipelines = $state(false);
  let errorMsg = $state("");
  let consecutiveFailures = $state(0);

  // Telemetry Payload data
  let telemetry = $state<TelemetryPayload | null>(null);

  // Inspector Drawer State
  let isInspectorOpen = $state(false);
  let inspectorJobName = $state("");
  let inspectorJobId = $state(0);
  let inspectorProjectPath = $state("");

  // MR Inspector Drawer State
  let isMRInspectorOpen = $state(false);
  let selectedMR = $state<MergeRequest | null>(null);

  function handleSelectMR(mr: MergeRequest) {
    selectedMR = mr;
    isMRInspectorOpen = true;
  }

  // Running polling timer reference
  let pollTimer: any = null;
  let isFetching = $state(false);
  // Timestamp (ms) of the last completed fetch — used for the focus-refresh cooldown
  let lastFetchedAt = 0;
  const FOCUS_COOLDOWN_MS = 60_000; // 60 seconds

  // Derived counts for Sidebar badges
  const todosCount = $derived(telemetry?.todos?.length || 0);
  const mrsCount = $derived(telemetry?.mergeRequests?.length || 0);
  const issuesCount = $derived(telemetry?.issues?.length || 0);
  
  const failedPipelines = $derived(
    ignoreFailedPipelines
      ? 0
      : (telemetry?.pipelines || []).filter((p) => p.pipeline?.status === "failed").length
  );
  
  const runningPipelines = $derived(
    (telemetry?.pipelines || []).filter(
      (p) => p.pipeline?.status === "running" || p.pipeline?.status === "pending"
    ).length
  );

  const username = $derived(telemetry?.username || "");
  const avatarUrl = $derived(telemetry?.avatarUrl || "");

  async function loadPollingSettings() {
    try {
      const cfg = await GetConfig();
      if (cfg) {
        pollIntervalSec = cfg.pollIntervalSec || 30;
        ignoreFailedPipelines = cfg.ignoreFailedPipelines || false;
      }
    } catch (e) {
      console.error("Failed to load config for polling:", e);
    }
  }

  // Polls backend for new telemetry data
  async function fetchTelemetryData(showLoader = false, force = false) {
    if (isFetching && !force) {
      console.log("[App] Telemetry fetch skipped (already in progress)");
      return;
    }
    isFetching = true;
    if (showLoader) {
      isLoading = true;
    }
    try {
      const data = await FetchTelemetry();
      if (data) {
        if (data.error === "unconfigured") {
          isConfigured = false;
          currentTab = "setup";
          telemetry = null;
        } else {
          isConfigured = true;
          telemetry = data;
          errorMsg = "";
          consecutiveFailures = 0;
        }
      }
    } catch (e: any) {
      console.error("Telemetry fetch error:", e);
      consecutiveFailures++;
      if (showLoader || consecutiveFailures >= 3) {
        errorMsg = e.message || "Unable to retrieve DevOps telemetry from GitLab.";
      }
    } finally {
      isLoading = false;
      isFetching = false;
      lastFetchedAt = Date.now();
    }
  }

  function startPolling() {
    stopPolling();
    
    async function tick() {
      await fetchTelemetryData(false);
      if (pollTimer !== null) {
        pollTimer = setTimeout(tick, pollIntervalSec * 1000);
      }
    }
    
    pollTimer = setTimeout(tick, pollIntervalSec * 1000);
  }

  function stopPolling() {
    if (pollTimer) {
      clearTimeout(pollTimer);
      pollTimer = null;
    }
  }

  // Triggered when settings are updated
  async function handleConfigSaved() {
    await loadPollingSettings();
    await fetchTelemetryData(true, true);
    if (isConfigured && currentTab === "setup") {
      currentTab = "todos"; // Auto switch to main view
    }
    startPolling(); // Restart poll with new duration
  }

  async function toggleIgnoreFailedPipelines() {
    try {
      const cfg = await GetConfig();
      if (cfg) {
        cfg.ignoreFailedPipelines = !cfg.ignoreFailedPipelines;
        await SaveConfig(cfg);
        ignoreFailedPipelines = cfg.ignoreFailedPipelines;
        await fetchTelemetryData(true, true);
      }
    } catch (e) {
      console.error("Failed to toggle ignore failed pipelines:", e);
    }
  }

  function handleSelectJobLog(projectPath: string, jobId: number, jobName: string) {
    inspectorProjectPath = projectPath;
    inspectorJobId = jobId;
    inspectorJobName = jobName;
    isInspectorOpen = true;
  }

  async function updateMaximisedState() {
    try {
      isMaximised = await Window.IsMaximised();
    } catch (err) {
      console.warn("Failed to fetch maximised state:", err);
    }
  }

  async function handleToggleMaximize() {
    await Window.ToggleMaximise();
    await updateMaximisedState();
  }

  function handleDoubleClickTitlebar() {
    handleToggleMaximize();
  }

  onMount(async () => {
    const ua = navigator.userAgent.toLowerCase();
    isWindows = ua.includes("windows") || navigator.platform.toLowerCase().includes("win");
    console.log("[App] isWindows:", isWindows, "ua:", navigator.userAgent, "platform:", navigator.platform);
    if (isWindows) {
      window.addEventListener("resize", updateMaximisedState);
      updateMaximisedState();
    }

    // 1. Load config settings first
    await loadPollingSettings();

    // 2. Load cached telemetry data instantly
    try {
      const cachedData = await GetCachedTelemetry();
      if (cachedData && cachedData.username) {
        telemetry = cachedData;
        isConfigured = true;
      }
    } catch (e) {
      console.warn("[App] Failed to load cached telemetry:", e);
    }

    // 3. Sync fresh telemetry in the background.
    // If we have cached data, we don't block the UI with the full-screen loader.
    // If there is no cached data (e.g. cold start / first setup), show the spinner.
    const showLoader = !telemetry;
    fetchTelemetryData(showLoader);
    startPolling();
  });

  function handleWindowFocus() {
    const msSinceLast = Date.now() - lastFetchedAt;
    if (msSinceLast < FOCUS_COOLDOWN_MS) {
      console.log(`[App] Window focused, skipping refresh (last fetch ${Math.round(msSinceLast / 1000)}s ago < ${FOCUS_COOLDOWN_MS / 1000}s cooldown)`);
      return;
    }
    console.log("[App] Window focused, refreshing telemetry...");
    fetchTelemetryData(false);
  }

  onDestroy(() => {
    stopPolling();
    if (isWindows) {
      window.removeEventListener("resize", updateMaximisedState);
    }
  });
</script>

<svelte:window onfocus={handleWindowFocus} />

<div class="app-container select-none">
  <!-- Sidebar Panel -->
  <Sidebar
    {currentTab}
    {todosCount}
    {runningPipelines}
    {failedPipelines}
    {mrsCount}
    {issuesCount}
    {username}
    {avatarUrl}
    syncError={errorMsg}
    isSyncing={isFetching}
    onSelectTab={(tab) => (currentTab = tab)}
  />

  <!-- Main Workspace Area -->
  <main class="h-screen overflow-hidden flex flex-col bg-slate-950/45 text-slate-100 relative">
    
    <!-- Ambient background glows for premium glassmorphism -->
    <div class="absolute top-[-15%] left-[-10%] w-[65%] h-[65%] rounded-full bg-indigo-600/8 blur-[130px] pointer-events-none z-0"></div>
    <div class="absolute bottom-[-10%] right-[-10%] w-[55%] h-[55%] rounded-full bg-emerald-600/4 blur-[120px] pointer-events-none z-0"></div>

    <!-- Title bar drag area (required for chromeless hidden-inset windows on macOS) -->
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="h-10 shrink-0 select-none cursor-default flex items-center justify-between px-4 border-b border-slate-900/10 relative z-20"
      style="-webkit-app-region: drag; --wails-draggable: drag;"
      role="none"
      ondblclick={handleDoubleClickTitlebar}
    >
      <!-- Title on Windows -->
      <div class="text-[10px] font-mono text-slate-500 font-bold uppercase tracking-wider select-none pointer-events-none">
        {#if isWindows}
          Gittar Control Panel
        {/if}
      </div>

      <!-- Custom Fluent Windows Titlebar Controls -->
      {#if isWindows}
        <div class="flex items-center h-full -mr-4" style="-webkit-app-region: no-drag; --wails-draggable: no-drag;">
          <!-- Minimize -->
          <button
            type="button"
            aria-label="Minimize"
            onclick={() => Window.Minimise()}
            class="h-10 w-12 flex items-center justify-center text-slate-400 hover:text-slate-100 hover:bg-white/10 transition-colors duration-150 cursor-pointer"
            title="Minimize"
          >
            <svg class="w-3.5 h-3.5" viewBox="0 0 10.2 1" fill="none" stroke="currentColor" stroke-width="1.2">
              <line x1="0" y1="0.5" x2="10.2" y2="0.5" />
            </svg>
          </button>
          
          <!-- Maximize / Restore -->
          <button
            type="button"
            aria-label={isMaximised ? "Restore" : "Maximize"}
            onclick={handleToggleMaximize}
            class="h-10 w-12 flex items-center justify-center text-slate-400 hover:text-slate-100 hover:bg-white/10 transition-colors duration-150 cursor-pointer"
            title={isMaximised ? "Restore" : "Maximize"}
          >
            {#if isMaximised}
              <!-- Restore icon (double square) -->
              <svg class="w-3.5 h-3.5" viewBox="0 0 10.2 10.2" fill="none" stroke="currentColor" stroke-width="1.2">
                <path d="M2.1,2.1 L2.1,0.5 L9.7,0.5 L9.7,8.1 L8.1,8.1" />
                <rect x="0.5" y="2.1" width="7.6" height="7.6" />
              </svg>
            {:else}
              <!-- Maximize icon (single square) -->
              <svg class="w-2.5 h-2.5" viewBox="0 0 10.2 10.2" fill="none" stroke="currentColor" stroke-width="1.2">
                <rect x="0.5" y="0.5" width="9.2" height="9.2" />
              </svg>
            {/if}
          </button>
          
          <!-- Close -->
          <button
            type="button"
            aria-label="Close"
            onclick={() => Window.Close()}
            class="h-10 w-12 flex items-center justify-center text-slate-400 hover:text-white hover:bg-rose-600 transition-colors duration-150 cursor-pointer"
            title="Close"
          >
            <svg class="w-3.5 h-3.5" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="1.2">
              <path d="M1,1 L9,9 M9,1 L1,9" />
            </svg>
          </button>
        </div>
      {/if}
    </div>

    <!-- Network Loader -->
    {#if isLoading}
      <div class="absolute inset-0 bg-slate-950/60 z-30 flex items-center justify-center space-x-3 backdrop-blur-[1px]">
        <div class="w-7 h-7 border-2 border-indigo-500 border-t-transparent rounded-full animate-spin"></div>
        <span class="text-slate-300 text-xs font-mono font-medium">Synchronizing telemetry...</span>
      </div>
    {/if}

    <!-- Content Router -->
    <div class="flex-1 overflow-hidden relative z-10">

      {#if currentTab === "todos"}
        <TodosPanel 
          todos={telemetry?.todos || []} 
          onRefresh={() => fetchTelemetryData(false)}
        />
      {:else if currentTab === "pipelines"}
        <PipelinesPanel
          pipelines={telemetry?.pipelines || []}
          onSelectJobLog={handleSelectJobLog}
          onRefresh={() => fetchTelemetryData(false)}
          {ignoreFailedPipelines}
          onToggleIgnoreFailed={toggleIgnoreFailedPipelines}
        />
      {:else if currentTab === "mrs"}
        <MRsPanel
          mergeRequests={telemetry?.mergeRequests || []}
          {username}
          onRefresh={() => fetchTelemetryData(false)}
          onSelectMR={handleSelectMR}
        />
      {:else if currentTab === "issues"}
        <IssuesPanel
          issues={telemetry?.issues || []}
          {username}
          onRefresh={() => fetchTelemetryData(false)}
        />
      {:else if currentTab === "setup"}
        <div class="h-full overflow-y-auto p-6">
          <SetupPanel onConfigSaved={handleConfigSaved} />
        </div>
      {/if}
    </div>
  </main>

  <!-- Sliding Drawer Inspector Panel -->
  <InspectorPanel
    isOpen={isInspectorOpen}
    jobName={inspectorJobName}
    jobId={inspectorJobId}
    projectPath={inspectorProjectPath}
    onClose={() => (isInspectorOpen = false)}
  />

  <!-- Sliding Drawer MR Inspector Panel -->
  <MRInspectorPanel
    isOpen={isMRInspectorOpen}
    mr={selectedMR}
    {username}
    onClose={() => (isMRInspectorOpen = false)}
    onRefreshList={() => fetchTelemetryData(false)}
  />
</div>
