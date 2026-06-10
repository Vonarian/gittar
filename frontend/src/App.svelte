<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Window } from "@wailsio/runtime";
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
    }
  }

  function startPolling() {
    stopPolling();
    // Schedule interval
    pollTimer = setInterval(() => {
      fetchTelemetryData(false);
    }, pollIntervalSec * 1000);
  }

  function stopPolling() {
    if (pollTimer) {
      clearInterval(pollTimer);
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

  function handleDoubleClickTitlebar() {
    Window.ToggleMaximise();
  }

  onMount(async () => {
    isLoading = true;
    await loadPollingSettings();
    fetchTelemetryData(true);
    startPolling();
  });

  onDestroy(() => {
    stopPolling();
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
      class="h-10 shrink-0 select-none cursor-default"
      style="-webkit-app-region: drag"
      role="none"
      ondblclick={handleDoubleClickTitlebar}
    ></div>

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
