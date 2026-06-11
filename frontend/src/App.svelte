<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Window, System } from "@wailsio/runtime";
  import Sidebar from "./lib/components/Sidebar.svelte";
  import SetupPanel from "./lib/components/SetupPanel.svelte";
  import TodosPanel from "./lib/components/TodosPanel.svelte";
  import PipelinesPanel from "./lib/components/PipelinesPanel.svelte";
  import MRsPanel from "./lib/components/MRsPanel.svelte";
  import InspectorPanel from "./lib/components/InspectorPanel.svelte";

  import { FetchTelemetry, GetConfig } from "../bindings/gittar/internal/service/appservice";
  import type { TelemetryPayload } from "../bindings/gittar/internal/gitlab/models";

  // Reactive state using Svelte 5 Runes
  let currentTab = $state("todos");
  let isWindows = $state(false);
  let isMaximised = $state(false);
  let isConfigured = $state(true);
  let isLoading = $state(false);
  let pollIntervalSec = $state(30);
  let errorMsg = $state("");
  let consecutiveFailures = $state(0);

  // Telemetry Payload data
  let telemetry = $state<TelemetryPayload | null>(null);

  // Inspector Drawer State
  let isInspectorOpen = $state(false);
  let inspectorJobName = $state("");
  let inspectorJobId = $state(0);
  let inspectorProjectPath = $state("");

  // Running polling timer reference
  let pollTimer: any = null;
  let isFetching = false;

  // Derived counts for Sidebar badges
  const todosCount = $derived(telemetry?.todos?.length || 0);
  const mrsCount = $derived(telemetry?.mergeRequests?.length || 0);
  
  const failedPipelines = $derived(
    (telemetry?.pipelines || []).filter((p) => p.pipeline?.status === "failed").length
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
      }
    } catch (e) {
      console.error("Failed to load config for polling:", e);
    }
  }

  // Polls backend for new telemetry data
  async function fetchTelemetryData(showLoader = false) {
    if (isFetching) {
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
    await fetchTelemetryData(true);
    if (isConfigured && currentTab === "setup") {
      currentTab = "todos"; // Auto switch to main view
    }
    startPolling(); // Restart poll with new duration
  }

  function handleSelectJobLog(projectPath: string, jobId: number, jobName: string) {
    inspectorProjectPath = projectPath;
    inspectorJobId = jobId;
    inspectorJobName = jobName;
    isInspectorOpen = true;
  }

  function updateMaximisedState() {
    Window.IsMaximised().then((state) => {
      isMaximised = state;
    });
  }

  async function handleToggleMaximize() {
    await Window.ToggleMaximise();
    updateMaximisedState();
  }

  function handleDoubleClickTitlebar() {
    handleToggleMaximize();
  }

  onMount(async () => {
    isLoading = true;
    isWindows = System.IsWindows();
    if (isWindows) {
      window.addEventListener("resize", updateMaximisedState);
      updateMaximisedState();
    }
    await loadPollingSettings();
    fetchTelemetryData(true);
    startPolling();
  });

  onDestroy(() => {
    stopPolling();
    if (isWindows) {
      window.removeEventListener("resize", updateMaximisedState);
    }
  });
</script>

<div class="app-container select-none">
  <!-- Sidebar Panel -->
  <Sidebar
    {currentTab}
    {todosCount}
    {runningPipelines}
    {failedPipelines}
    {mrsCount}
    {username}
    {avatarUrl}
    syncError={errorMsg}
    onSelectTab={(tab) => (currentTab = tab)}
  />

  <!-- Main Workspace Area -->
  <main class="h-screen overflow-hidden flex flex-col bg-slate-950/45 text-slate-100 relative">
    
    <!-- Title bar drag area (required for chromeless hidden-inset windows on macOS) -->
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="h-10 shrink-0 select-none cursor-default flex items-center justify-between px-4 border-b border-slate-900/10"
      style="-webkit-app-region: drag"
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
        <div class="flex items-center h-full -mr-4" style="-webkit-app-region: no-drag">
          <!-- Minimize -->
          <button
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
    <div class="flex-1 overflow-hidden">

      {#if currentTab === "todos"}
        <TodosPanel todos={telemetry?.todos || []} />
      {:else if currentTab === "pipelines"}
        <PipelinesPanel
          pipelines={telemetry?.pipelines || []}
          onSelectJobLog={handleSelectJobLog}
        />
      {:else if currentTab === "mrs"}
        <MRsPanel
          mergeRequests={telemetry?.mergeRequests || []}
          {username}
          onRefresh={() => fetchTelemetryData(true)}
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
</div>
