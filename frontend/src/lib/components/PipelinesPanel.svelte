<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import type { PipelineWithJobs, Job } from "../../../bindings/gittar/internal/gitlab/models";
  import { Browser, Clipboard } from "@wailsio/runtime";
  import { RetryPipeline, CancelPipeline } from "../../../bindings/gittar/internal/service/appservice";

  interface Props {
    pipelines: PipelineWithJobs[];
    onSelectJobLog: (projectPath: string, jobId: number, jobName: string) => void;
    onRefresh?: () => void;
    ignoreFailedPipelines?: boolean;
    onToggleIgnoreFailed?: () => void;
  }

  let {
    pipelines = [],
    onSelectJobLog,
    onRefresh,
    ignoreFailedPipelines = false,
    onToggleIgnoreFailed
  }: Props = $props();

  let viewMode = $state("list");



  // Loading indicator for pipeline actions
  let actionLoading = $state<Record<number, boolean>>({});

  // Context menu state
  let contextMenu = $state<{ x: number; y: number; link: string } | null>(null);

  function handleContextMenu(e: MouseEvent, link: string) {
    if (!link) return;
    e.preventDefault();
    e.stopPropagation();
    contextMenu = {
      x: e.clientX,
      y: e.clientY,
      link
    };
  }

  function closeContextMenu() {
    contextMenu = null;
  }

  // Search & Filter state variables
  let searchQuery = $state("");
  let groupFilter = $state("all");
  let projectFilter = $state("all");
  let statusFilter = $state("all");

  onMount(() => {
    viewMode = localStorage.getItem("gittar_view_mode_pipelines") || "list";
    searchQuery = localStorage.getItem("gittar_filter_pipelines_search") || "";
    groupFilter = localStorage.getItem("gittar_filter_pipelines_group") || "all";
    projectFilter = localStorage.getItem("gittar_filter_pipelines_project") || "all";
    statusFilter = localStorage.getItem("gittar_filter_pipelines_status") || "all";

    window.addEventListener("click", closeContextMenu);
    window.addEventListener("contextmenu", closeContextMenu);
  });

  onDestroy(() => {
    window.removeEventListener("click", closeContextMenu);
    window.removeEventListener("contextmenu", closeContextMenu);
  });

  $effect(() => {
    localStorage.setItem("gittar_view_mode_pipelines", viewMode);
  });

  $effect(() => {
    localStorage.setItem("gittar_filter_pipelines_search", searchQuery);
    localStorage.setItem("gittar_filter_pipelines_group", groupFilter);
    localStorage.setItem("gittar_filter_pipelines_project", projectFilter);
    localStorage.setItem("gittar_filter_pipelines_status", statusFilter);
  });

  // Extract unique group names and project names dynamically
  const uniqueGroups = $derived([
    ...new Set(pipelines.map((p) => p.projectPath?.split("/")[0]).filter(Boolean)),
  ]);

  const uniqueProjects = $derived([
    ...new Set(pipelines.map((p) => p.projectName).filter(Boolean)),
  ]);

  // Derived filtered pipelines
  const filteredPipelines = $derived(
    pipelines.filter((pwj) => {
      const hasNoPipeline = !pwj.pipeline || pwj.pipeline.id === 0;

      const matchesSearch =
        searchQuery === "" ||
        (pwj.projectName || "").toLowerCase().includes(searchQuery.toLowerCase()) ||
        (pwj.pipeline?.ref || "").toLowerCase().includes(searchQuery.toLowerCase()) ||
        (pwj.pipeline?.sha || "").toLowerCase().includes(searchQuery.toLowerCase());

      const matchesGroup =
        groupFilter === "all" || pwj.projectPath?.split("/")[0] === groupFilter;

      const matchesProject =
        projectFilter === "all" || pwj.projectName === projectFilter;

      let matchesStatus = false;
      if (statusFilter === "all") {
        matchesStatus = true;
      } else if (statusFilter === "none") {
        matchesStatus = hasNoPipeline;
      } else if (!hasNoPipeline) {
        if (statusFilter === "running") {
          matchesStatus = pwj.pipeline.status === "running" || pwj.pipeline.status === "pending";
        } else {
          matchesStatus = pwj.pipeline.status === statusFilter;
        }
      }

      return matchesSearch && matchesGroup && matchesProject && matchesStatus;
    })
  );

  // Kanban Derived Columns
  const pendingPipelines = $derived(
    filteredPipelines.filter(
      (pwj) =>
        pwj.pipeline?.id > 0 &&
        (pwj.pipeline.status === "pending" ||
          pwj.pipeline.status === "created" ||
          pwj.pipeline.status === "scheduled")
    )
  );

  const runningPipelines = $derived(
    filteredPipelines.filter((pwj) => pwj.pipeline?.id > 0 && pwj.pipeline.status === "running")
  );

  const successPipelines = $derived(
    filteredPipelines.filter((pwj) => pwj.pipeline?.id > 0 && pwj.pipeline.status === "success")
  );

  const failedPipelines = $derived(
    filteredPipelines.filter((pwj) => pwj.pipeline?.id > 0 && pwj.pipeline.status === "failed")
  );

  const finishedOtherPipelines = $derived(
    filteredPipelines.filter(
      (pwj) =>
        pwj.pipeline?.id > 0 &&
        (pwj.pipeline.status === "canceled" ||
          pwj.pipeline.status === "skipped" ||
          pwj.pipeline.status === "manual")
    )
  );

  const noPipelines = $derived(filteredPipelines.filter((pwj) => !pwj.pipeline || pwj.pipeline.id === 0));

  function resetFilters() {
    searchQuery = "";
    groupFilter = "all";
    projectFilter = "all";
    statusFilter = "all";
  }

  // Pipeline Actions: Retry & Cancel
  async function handleRetry(projectPath: string, pipelineId: number) {
    if (actionLoading[pipelineId]) return;
    if (!confirm("Are you sure you want to retry this pipeline?")) {
      return;
    }
    actionLoading[pipelineId] = true;
    try {
      await RetryPipeline(projectPath, pipelineId);
      if (onRefresh) onRefresh();
    } catch (e: any) {
      console.error("Failed to retry pipeline:", e);
      alert("Failed to retry pipeline: " + e.message);
    } finally {
      actionLoading[pipelineId] = false;
    }
  }

  async function handleCancel(projectPath: string, pipelineId: number) {
    if (actionLoading[pipelineId]) return;
    if (!confirm("Are you sure you want to cancel this running pipeline?")) {
      return;
    }
    actionLoading[pipelineId] = true;
    try {
      await CancelPipeline(projectPath, pipelineId);
      if (onRefresh) onRefresh();
    } catch (e: any) {
      console.error("Failed to cancel pipeline:", e);
      alert("Failed to cancel pipeline: " + e.message);
    } finally {
      actionLoading[pipelineId] = false;
    }
  }



  // Color code status capsules
  function getPipelineStatusClasses(status: string): string {
    switch (status?.toLowerCase()) {
      case "success":
        return "bg-emerald-500/10 text-emerald-400 border-emerald-500/30";
      case "failed":
        return "bg-rose-500/10 text-rose-400 border-rose-500/30";
      case "running":
      case "pending":
        return "bg-amber-500/10 text-amber-400 border-amber-500/30";
      case "canceled":
      case "skipped":
        return "bg-slate-500/10 text-slate-400 border-slate-500/30";
      default:
        return "bg-slate-700/10 text-slate-400 border-slate-700/30";
    }
  }

  // Format pipeline duration: e.g. 132 -> "2m 12s"
  function formatDuration(sec: number): string {
    if (!sec || sec <= 0) return "0s";
    const mins = Math.floor(sec / 60);
    const secs = sec % 60;
    if (mins > 0) return `${mins}m ${secs}s`;
    return `${secs}s`;
  }

  function formatSHA(sha: string): string {
    if (!sha) return "";
    return sha.substring(0, 8);
  }

  // Group jobs by stage name
  function groupJobsByStage(jobs: Job[]): Record<string, Job[]> {
    const groups: Record<string, Job[]> = {};
    for (const job of jobs || []) {
      const stage = job.stage || "other";
      if (!groups[stage]) {
        groups[stage] = [];
      }
      groups[stage].push(job);
    }
    return groups;
  }

  function getBorderLeftAccent(status: string): string {
    switch (status?.toLowerCase()) {
      case "success":
        return "border-l-emerald-500/50";
      case "failed":
        return "border-l-rose-500/50";
      case "running":
      case "pending":
        return "border-l-amber-500/50";
      case "canceled":
      case "skipped":
        return "border-l-slate-700/40";
      default:
        return "border-l-slate-800/60";
    }
  }
</script>

<div class="h-full flex flex-col">
  <!-- Panel Header -->
  <div class="p-6 border-b border-slate-900/60 flex items-center justify-between">
    <div>
      <h2 class="text-xl font-semibold text-white">Pipelines Matrix</h2>
      <p class="text-slate-400 text-xs mt-1">Real-time compilation and deployment matrices across active workspaces.</p>
    </div>

    <div class="flex items-center space-x-4">
      <!-- Layout Toggle Options -->
      <div class="flex items-center bg-slate-950/60 border border-slate-900 rounded-lg p-0.5 shadow-inner">
        <button
          onclick={() => (viewMode = "list")}
          class="px-3 py-1.5 text-xs font-semibold rounded-md transition flex items-center space-x-1.5 {viewMode === 'list' ? 'bg-indigo-650 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200'}"
        >
          <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
          </svg>
          <span>List</span>
        </button>
        <button
          onclick={() => (viewMode = "kanban")}
          class="px-3 py-1.5 text-xs font-semibold rounded-md transition flex items-center space-x-1.5 {viewMode === 'kanban' ? 'bg-indigo-650 text-white shadow-sm' : 'text-slate-400 hover:text-slate-200'}"
        >
          <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 17V7m0 10a2 2 0 01-2 2H5a2 2 0 01-2-2V7a2 2 0 012-2h2a2 2 0 012 2m0 10a2 2 0 002 2h2a2 2 0 002-2M9 7a2 2 0 012-2h2a2 2 0 012 2m0 10V7m0 10a2 2 0 002 2h2a2 2 0 002-2V7a2 2 0 00-2-2h-2a2 2 0 00-2 2" />
          </svg>
          <span>Kanban</span>
        </button>
      </div>
    </div>
  </div>

  <!-- Filter & Search Toolbar -->
  {#if pipelines.length > 0}
    <div class="px-6 py-3 border-b border-slate-900/40 bg-slate-950/20 flex flex-wrap items-center justify-between gap-3 select-none">
      <div class="flex flex-wrap items-center gap-3">
        <!-- Search Input -->
        <div class="relative w-48">
          <input
            type="text"
            bind:value={searchQuery}
            placeholder="Search pipelines..."
            class="w-full px-2.5 py-1.5 bg-slate-950 border border-slate-800 focus:border-indigo-600 rounded-lg text-xs text-slate-200 outline-none transition"
          />
        </div>

        <!-- Group Filter -->
        <select
          bind:value={groupFilter}
          class="px-2 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-xs text-slate-300 outline-none cursor-pointer focus:border-indigo-650"
        >
          <option value="all">All Groups</option>
          {#each uniqueGroups as grp (grp)}
            <option value={grp}>{grp}</option>
          {/each}
        </select>

        <!-- Project Filter -->
        <select
          bind:value={projectFilter}
          class="px-2 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-xs text-slate-300 outline-none cursor-pointer focus:border-indigo-650"
        >
          <option value="all">All Projects</option>
          {#each uniqueProjects as name (name)}
            <option value={name}>{name}</option>
          {/each}
        </select>

        <!-- Status Filter -->
        <select
          bind:value={statusFilter}
          class="px-2 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-xs text-slate-300 outline-none cursor-pointer focus:border-indigo-650"
        >
          <option value="all">All Statuses</option>
          <option value="success">Success</option>
          <option value="failed">Failed</option>
          <option value="running">Running / Pending</option>
          <option value="canceled">Canceled</option>
          <option value="skipped">Skipped</option>
          <option value="none">No Pipelines</option>
        </select>

        <!-- Reset Filters Button -->
        {#if searchQuery !== "" || groupFilter !== "all" || projectFilter !== "all" || statusFilter !== "all"}
          <button
            onclick={resetFilters}
            class="px-3 py-1.5 border border-indigo-500/30 hover:border-indigo-550/40 bg-indigo-550/10 hover:bg-indigo-550/20 text-indigo-400 text-xs font-semibold rounded-lg transition"
          >
            Reset Filters
          </button>
        {/if}
      </div>

      <!-- Ignore Failed Pipelines Checkbox Toggle (Persistent Connection Setting) -->
      <div class="flex items-center space-x-2 bg-slate-900/40 border border-slate-800/80 px-3 py-1.5 rounded-lg select-none">
        <label for="ignore-failed-pipelines" class="flex items-center space-x-2.5 text-xs text-slate-300 cursor-pointer select-none">
          <input
            id="ignore-failed-pipelines"
            type="checkbox"
            checked={ignoreFailedPipelines}
            onclick={onToggleIgnoreFailed}
            class="rounded border-slate-800 bg-slate-950 text-indigo-600 focus:ring-0 focus:ring-offset-0"
          />
          <span>Ignore Failed Pipelines count</span>
        </label>
      </div>
    </div>
  {/if}

  <!-- Content Area -->
  <div class="flex-1 overflow-hidden relative">
    {#if pipelines.length === 0}
      <div class="h-[70%] flex flex-col items-center justify-center text-center">
        <div class="w-12 h-12 rounded-full bg-slate-950/40 border border-slate-900 flex items-center justify-center text-slate-500 mb-4">
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
          </svg>
        </div>
        <h3 class="text-base font-semibold text-slate-200 font-sans">No Monitored Projects</h3>
        <p class="text-slate-500 text-sm mt-1 max-w-[280px]">Add project paths or group IDs under Connection Settings to display build pipelines.</p>
      </div>
    {:else if filteredPipelines.length === 0 && viewMode === "list"}
      <!-- Empty Filter State -->
      <div class="h-[70%] flex flex-col items-center justify-center text-center">
        <div class="w-12 h-12 rounded-full bg-slate-950/40 border border-slate-900 flex items-center justify-center text-slate-500 mb-4">
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
          </svg>
        </div>
        <h3 class="text-base font-semibold text-slate-350">No Results Found</h3>
        <p class="text-slate-500 text-sm mt-1 max-w-[280px]">No build matrices match your filter criteria. Try resetting or adjusting your search.</p>
        <button
          onclick={resetFilters}
          class="mt-4 px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-lg transition"
        >
          Reset Filters
        </button>
      </div>
    {:else if viewMode === "list"}
      <div class="h-full overflow-y-auto p-6 space-y-6">
        {#each filteredPipelines as pwj (pwj.pipeline?.id || pwj.projectPath)}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div
            oncontextmenu={(e) => { if (pwj.pipeline && pwj.pipeline.id > 0) handleContextMenu(e, pwj.pipeline.web_url); }}
            class="bg-slate-950/20 border-t border-r border-b border-l-2 border-y-slate-900/40 border-r-slate-900/40 {getBorderLeftAccent(pwj.pipeline?.status || '')} rounded-xl p-5 hover:bg-slate-900/25 hover:border-y-slate-800/60 hover:border-r-slate-800/60 hover:shadow-md hover:shadow-indigo-500/5 transition-all duration-200"
          >
            <!-- Project Info & Pipeline Header -->
            <div class="flex items-start justify-between flex-wrap gap-4">
              <div class="space-y-1">
                <div class="flex items-center space-x-2.5">
                  <h3 class="text-base font-semibold text-slate-100">{pwj.projectName}</h3>
                  <span class="text-xs font-mono text-slate-500">({pwj.projectPath})</span>
                  {#if pwj.pipeline?.ref}
                    <span class="px-1.5 py-0.5 text-[9px] font-bold tracking-wider bg-slate-900/60 text-indigo-400 border border-slate-800/80 rounded font-mono">
                      {pwj.pipeline.ref}
                    </span>
                  {/if}
                </div>
                
                {#if pwj.pipeline?.id > 0}
                  <!-- Commit Meta -->
                  <div class="flex items-center space-x-3 text-xs text-slate-400">
                    <span class="flex items-center space-x-1">
                      <svg class="w-3.5 h-3.5 text-slate-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
                      </svg>
                      <span class="font-mono bg-slate-955/65 px-1.5 py-0.5 rounded border border-slate-800 text-slate-300">
                        {formatSHA(pwj.pipeline.sha)}
                      </span>
                    </span>
                    <span class="text-slate-650">•</span>
                    <span class="flex items-center space-x-1">
                      <svg class="w-3.5 h-3.5 text-slate-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7a4 4 0 118 0 4 4 0 01-8 0zM2 17a10 10 0 1120 0H2z" />
                      </svg>
                      <span>{pwj.pipeline.user?.name || "GitLab Runner"}</span>
                    </span>
                    {#if pwj.pipeline.duration > 0}
                      <span class="text-slate-650">•</span>
                      <span class="flex items-center space-x-1">
                        <svg class="w-3.5 h-3.5 text-slate-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                        <span>{formatDuration(pwj.pipeline.duration)}</span>
                      </span>
                    {/if}
                  </div>
                {:else}
                  <p class="text-xs text-slate-500">No pipelines found on this project.</p>
                {/if}
              </div>

              <!-- Pipeline Status Capsule & Actions -->
              {#if pwj.pipeline?.id > 0}
                <div class="flex items-center space-x-3">
                  {#if pwj.pipeline.status === "failed" || pwj.pipeline.status === "canceled"}
                    <button
                      onclick={(e) => { e.stopPropagation(); handleRetry(pwj.projectPath, pwj.pipeline.id); }}
                      disabled={actionLoading[pwj.pipeline.id]}
                      class="px-2.5 py-1.5 bg-indigo-600/10 hover:bg-indigo-600 border border-indigo-500/30 text-indigo-400 hover:text-white rounded-lg text-xs font-semibold transition"
                      title="Retry Pipeline"
                    >
                      Retry
                    </button>
                  {/if}

                  {#if pwj.pipeline.status === "running" || pwj.pipeline.status === "pending"}
                    <button
                      onclick={(e) => { e.stopPropagation(); handleCancel(pwj.projectPath, pwj.pipeline.id); }}
                      disabled={actionLoading[pwj.pipeline.id]}
                      class="px-2.5 py-1.5 bg-rose-600/10 hover:bg-rose-600 border border-rose-500/30 text-rose-450 hover:text-white rounded-lg text-xs font-semibold transition"
                      title="Cancel Pipeline"
                    >
                      Cancel
                    </button>
                  {/if}

                  <div class="px-3 py-1 text-xs font-bold uppercase tracking-wider rounded-full border flex items-center space-x-2 {getPipelineStatusClasses(pwj.pipeline.status)}">
                    <span class="relative flex h-2 w-2">
                      {#if pwj.pipeline.status === "running" || pwj.pipeline.status === "pending"}
                        <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75"></span>
                        <span class="relative inline-flex rounded-full h-2 w-2 bg-amber-500"></span>
                      {:else if pwj.pipeline.status === "success"}
                        <span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                      {:else if pwj.pipeline.status === "failed"}
                        <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-rose-400 opacity-30"></span>
                        <span class="relative inline-flex rounded-full h-2 w-2 bg-rose-500"></span>
                      {:else}
                        <span class="relative inline-flex rounded-full h-2 w-2 bg-slate-400"></span>
                      {/if}
                    </span>
                    <span>{pwj.pipeline.status}</span>
                  </div>

                  <button
                    onclick={(e) => { e.stopPropagation(); Browser.OpenURL(pwj.pipeline.web_url); }}
                    class="p-1.5 border border-slate-800 hover:border-slate-700 bg-slate-950/44 rounded-lg text-slate-400 hover:text-slate-200 transition"
                    title="Open Pipeline in Browser"
                  >
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                    </svg>
                  </button>
                </div>
              {/if}
            </div>

            <!-- Jobs Matrix Breakdown -->
            {#if pwj.jobs && pwj.jobs.length > 0}
              <div class="mt-5 border-t border-slate-900/65 pt-4">
                <h4 class="text-xs font-semibold uppercase tracking-wider text-slate-500 mb-3">Pipeline Stages</h4>
                
                <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                  {#each Object.entries(groupJobsByStage(pwj.jobs)) as [stage, stageJobs] (stage)}
                    <div class="bg-slate-955/30 border border-slate-900/60 rounded-lg p-3">
                      <h5 class="text-xs font-bold text-slate-400 border-b border-slate-900/50 pb-1.5 mb-2 truncate capitalize">
                        {stage}
                      </h5>
                      
                      <div class="space-y-2">
                        {#each stageJobs as job (job.id)}
                          <!-- svelte-ignore a11y_no_static_element_interactions -->
                          <div
                            oncontextmenu={(e) => { e.stopPropagation(); handleContextMenu(e, job.web_url); }}
                            class="flex items-center justify-between text-xs min-w-0"
                          >
                            <div class="flex items-center space-x-2 min-w-0 pr-2">
                              {#if job.status === "success"}
                                <svg class="w-3.5 h-3.5 text-emerald-500 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M5 13l4 4L19 7" />
                                </svg>
                              {:else if job.status === "failed"}
                                <svg class="w-3.5 h-3.5 text-rose-500 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M6 18L18 6M6 6l12 12" />
                                </svg>
                              {:else if job.status === "running" || job.status === "pending"}
                                <div class="w-3 h-3 border-2 border-amber-500 border-t-transparent rounded-full animate-spin shrink-0"></div>
                              {:else}
                                <svg class="w-3.5 h-3.5 text-slate-500 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
                                </svg>
                              {/if}

                              <!-- svelte-ignore a11y_click_events_have_key_events -->
                              <!-- svelte-ignore a11y_no_static_element_interactions -->
                              <span class="text-slate-355 truncate font-medium hover:text-indigo-400 cursor-pointer transition" onclick={() => Browser.OpenURL(job.web_url)} title={job.name}>{job.name}</span>
                            </div>

                            <div class="flex items-center space-x-2 shrink-0">
                              {#if job.status === "failed"}
                                <button
                                  onclick={(e) => { e.stopPropagation(); onSelectJobLog(pwj.projectPath, job.id, job.name); }}
                                  class="px-2 py-0.5 bg-rose-500/10 hover:bg-rose-500/20 text-rose-400 hover:text-rose-300 border border-rose-500/25 rounded text-[10px] font-semibold transition"
                                >
                                  Logs
                                </button>
                              {/if}

                              <button
                                onclick={(e) => { e.stopPropagation(); Browser.OpenURL(job.web_url); }}
                                class="text-slate-500 hover:text-slate-300 transition"
                                title="Open Job details"
                              >
                                <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                                </svg>
                              </button>
                            </div>
                          </div>
                        {/each}
                      </div>
                    </div>
                  {/each}
                </div>
              </div>
            {/if}
          </div>
        {/each}
      </div>
    {:else}
      <!-- Kanban Board View -->
      <div class="h-full w-full overflow-x-auto p-6 flex space-x-4 select-none">
        
        <!-- Column 1: Created / Pending -->
        <div class="flex-1 min-w-[260px] max-w-[320px] bg-slate-900/15 border border-slate-900/80 rounded-2xl p-3.5 flex flex-col h-full">
          <div class="flex items-center justify-between pb-2 mb-3 border-b border-slate-900/50 shrink-0">
            <h3 class="text-xs font-bold uppercase tracking-wider text-slate-400">Pending</h3>
            <span class="px-1.5 py-0.5 text-[10px] font-bold bg-slate-800 text-slate-400 rounded-full">{pendingPipelines.length}</span>
          </div>
          <div class="flex-1 overflow-y-auto space-y-2.5 pr-1">
            {#each pendingPipelines as pwj (pwj.pipeline?.id || pwj.projectPath)}
              <div
                class="group p-3 bg-slate-950/35 hover:bg-slate-950/60 border border-slate-900/60 hover:border-slate-800/70 rounded-xl transition"
              >
                <div class="text-[10px] font-bold text-indigo-400 truncate">{pwj.projectPath}</div>
                <h4 class="text-xs font-semibold text-slate-200 mt-1 truncate">{pwj.projectName}</h4>
                <div class="flex items-center justify-between mt-3 text-[10px] text-slate-500 font-mono">
                  <span>{formatSHA(pwj.pipeline.sha)}{#if pwj.pipeline?.ref} ({pwj.pipeline.ref}){/if}</span>
                  <span class="px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-400 border border-amber-500/20 uppercase text-[8px] font-bold">pending</span>
                </div>
              </div>
            {/each}
          </div>
        </div>

        <!-- Column 2: Running -->
        <div class="flex-1 min-w-[260px] max-w-[320px] bg-slate-900/15 border border-slate-900/80 rounded-2xl p-3.5 flex flex-col h-full">
          <div class="flex items-center justify-between pb-2 mb-3 border-b border-slate-900/50 shrink-0">
            <h3 class="text-xs font-bold uppercase tracking-wider text-amber-400">Running</h3>
            <span class="px-1.5 py-0.5 text-[10px] font-bold bg-amber-500/10 text-amber-400 rounded-full">{runningPipelines.length}</span>
          </div>
          <div class="flex-1 overflow-y-auto space-y-2.5 pr-1">
            {#each runningPipelines as pwj (pwj.pipeline?.id || pwj.projectPath)}
              <div
                class="group p-3 bg-slate-950/35 hover:bg-slate-950/60 border border-slate-900/60 hover:border-slate-800/70 rounded-xl transition"
              >
                <div class="text-[10px] font-bold text-indigo-400 truncate">{pwj.projectPath}</div>
                <h4 class="text-xs font-semibold text-slate-200 mt-1 truncate">{pwj.projectName}</h4>
                <div class="flex items-center justify-between mt-3 text-[10px] text-slate-500 font-mono">
                  <span>{formatSHA(pwj.pipeline.sha)}{#if pwj.pipeline?.ref} ({pwj.pipeline.ref}){/if}</span>
                  <div class="flex items-center space-x-2">
                    <button
                      onclick={(e) => { e.stopPropagation(); handleCancel(pwj.projectPath, pwj.pipeline.id); }}
                      disabled={actionLoading[pwj.pipeline.id]}
                      class="px-1.5 py-0.5 rounded bg-rose-650/10 hover:bg-rose-650 text-rose-400 hover:text-white border border-rose-500/25 hover:border-rose-500 uppercase text-[8px] font-bold transition"
                    >
                      cancel
                    </button>
                    <div class="w-2.5 h-2.5 border-2 border-amber-500 border-t-transparent rounded-full animate-spin"></div>
                  </div>
                </div>
              </div>
            {/each}
          </div>
        </div>

        <!-- Column 3: Passed -->
        <div class="flex-1 min-w-[260px] max-w-[320px] bg-slate-900/15 border border-slate-900/80 rounded-2xl p-3.5 flex flex-col h-full">
          <div class="flex items-center justify-between pb-2 mb-3 border-b border-slate-900/50 shrink-0">
            <h3 class="text-xs font-bold uppercase tracking-wider text-emerald-400">Passed</h3>
            <span class="px-1.5 py-0.5 text-[10px] font-bold bg-emerald-500/10 text-emerald-400 rounded-full">{successPipelines.length}</span>
          </div>
          <div class="flex-1 overflow-y-auto space-y-2.5 pr-1">
            {#each successPipelines as pwj (pwj.pipeline?.id || pwj.projectPath)}
              <div class="group p-3 bg-slate-950/20 border border-slate-900/40 rounded-xl opacity-80 hover:opacity-100 transition">
                <div class="text-[10px] font-bold text-indigo-400 truncate">{pwj.projectPath}</div>
                <h4 class="text-xs font-semibold text-slate-255 mt-1 truncate">{pwj.projectName}</h4>
                <div class="flex items-center justify-between mt-3 text-[10px] text-slate-500 font-mono">
                  <span>{formatSHA(pwj.pipeline.sha)}{#if pwj.pipeline?.ref} ({pwj.pipeline.ref}){/if}</span>
                  <span class="px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 uppercase text-[8px] font-bold">passed</span>
                </div>
              </div>
            {/each}
          </div>
        </div>

        <!-- Column 4: Failed -->
        <div class="flex-1 min-w-[260px] max-w-[320px] bg-slate-900/15 border border-slate-900/80 rounded-2xl p-3.5 flex flex-col h-full">
          <div class="flex items-center justify-between pb-2 mb-3 border-b border-slate-900/50 shrink-0">
            <h3 class="text-xs font-bold uppercase tracking-wider text-rose-450">Failed</h3>
            <span class="px-1.5 py-0.5 text-[10px] font-bold bg-rose-500/10 text-rose-400 rounded-full">{failedPipelines.length}</span>
          </div>
          <div class="flex-1 overflow-y-auto space-y-2.5 pr-1">
            {#each failedPipelines as pwj (pwj.pipeline?.id || pwj.projectPath)}
              <div
                class="group p-3 bg-slate-950/35 hover:bg-slate-950/60 border border-slate-900/60 hover:border-slate-800/70 rounded-xl transition"
              >
                <div class="text-[10px] font-bold text-indigo-400 truncate">{pwj.projectPath}</div>
                <h4 class="text-xs font-semibold text-slate-200 mt-1 truncate">{pwj.projectName}</h4>
                <div class="flex items-center justify-between mt-3 text-[10px] text-slate-500 font-mono">
                  <span>{formatSHA(pwj.pipeline.sha)}{#if pwj.pipeline?.ref} ({pwj.pipeline.ref}){/if}</span>
                  <button
                    onclick={(e) => { e.stopPropagation(); handleRetry(pwj.projectPath, pwj.pipeline.id); }}
                    disabled={actionLoading[pwj.pipeline.id]}
                    class="px-1.5 py-0.5 rounded bg-rose-650/10 hover:bg-rose-650 text-rose-400 hover:text-white border border-rose-500/25 hover:border-rose-500 uppercase text-[8px] font-bold transition"
                  >
                    failed (retry)
                  </button>
                </div>
              </div>
            {/each}
          </div>
        </div>

        <!-- Column 5: Finished Other -->
        <div class="flex-1 min-w-[260px] max-w-[320px] bg-slate-900/15 border border-slate-900/80 rounded-2xl p-3.5 flex flex-col h-full">
          <div class="flex items-center justify-between pb-2 mb-3 border-b border-slate-900/50 shrink-0">
            <h3 class="text-xs font-bold uppercase tracking-wider text-slate-450">Finished Other</h3>
            <span class="px-1.5 py-0.5 text-[10px] font-bold bg-slate-800 text-slate-400 rounded-full">{finishedOtherPipelines.length}</span>
          </div>
          <div class="flex-1 overflow-y-auto space-y-2.5 pr-1">
            {#each finishedOtherPipelines as pwj (pwj.pipeline?.id || pwj.projectPath)}
              <div
                class="group p-3 bg-slate-950/35 hover:bg-slate-950/60 border border-slate-900/60 hover:border-slate-800/70 rounded-xl transition"
              >
                <div class="text-[10px] font-bold text-indigo-400 truncate">{pwj.projectPath}</div>
                <h4 class="text-xs font-semibold text-slate-200 mt-1 truncate">{pwj.projectName}</h4>
                <div class="flex items-center justify-between mt-3 text-[10px] text-slate-500 font-mono">
                  <span>{formatSHA(pwj.pipeline.sha)}{#if pwj.pipeline?.ref} ({pwj.pipeline.ref}){/if}</span>
                  <span class="px-1.5 py-0.5 rounded bg-slate-800 text-slate-400 border border-slate-800 uppercase text-[8px] font-bold">{pwj.pipeline.status}</span>
                </div>
              </div>
            {/each}
          </div>
        </div>

        <!-- Column 6: No Pipelines -->
        {#if noPipelines.length > 0}
          <div class="flex-1 min-w-[260px] max-w-[320px] bg-slate-900/15 border border-slate-900/80 rounded-2xl p-3.5 flex flex-col h-full opacity-60">
            <div class="flex items-center justify-between pb-2 mb-3 border-b border-slate-900/50 shrink-0">
              <h3 class="text-xs font-bold uppercase tracking-wider text-slate-500">No Pipelines</h3>
              <span class="px-1.5 py-0.5 text-[10px] font-bold bg-slate-850 text-slate-500 rounded-full">{noPipelines.length}</span>
            </div>
            <div class="flex-1 overflow-y-auto space-y-2.5 pr-1">
              {#each noPipelines as pwj (pwj.projectPath)}
                <div class="p-3 bg-slate-955/10 border border-slate-900/40 rounded-xl">
                  <div class="text-[10px] font-bold text-slate-500 truncate">{pwj.projectPath}</div>
                  <h4 class="text-xs font-semibold text-slate-400 mt-1 truncate">{pwj.projectName}</h4>
                </div>
              {/each}
            </div>
          </div>
        {/if}

      </div>
    {/if}
  </div>
</div>

<!-- Custom Context Menu for Right-Click Link Copying -->
{#if contextMenu}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="fixed z-50 bg-slate-900 border border-slate-800 rounded-lg shadow-xl py-1 text-xs text-slate-200 min-w-[120px] select-none backdrop-blur-md"
    style="left: {contextMenu.x}px; top: {contextMenu.y}px;"
    onclick={(e) => e.stopPropagation()}
  >
    <button
      onclick={() => {
        Clipboard.SetText(contextMenu!.link);
        closeContextMenu();
      }}
      class="w-full text-left px-3 py-2 hover:bg-indigo-600 hover:text-white transition duration-150 flex items-center space-x-2"
    >
      <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
      </svg>
      <span>Copy Link</span>
    </button>
  </div>
{/if}
