<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import type { MergeRequest } from "../../../bindings/gittar/internal/gitlab/models";
  import { Browser, Clipboard } from "@wailsio/runtime";
  import { MergeMergeRequest, CloseMergeRequest } from "../../../bindings/gittar/internal/service/appservice";

  interface Props {
    mergeRequests: MergeRequest[];
    username: string;
    onRefresh?: () => void;
  }

  let { mergeRequests = [], username, onRefresh }: Props = $props();

  let activeMRTab = $state<"all" | "assigned" | "authored" | "review">("all");
  let viewMode = $state("list");

  // Search & Filter state variables
  let searchQuery = $state("");
  let groupFilter = $state("all");
  let projectFilter = $state("all");
  let userFilter = $state("all");

  // Drag-and-drop state
  let draggedMR = $state<MergeRequest | null>(null);
  let isDraggingOverMerge = $state(false);
  let isDraggingOverClose = $state(false);

  // Local state for processing actions (e.g. merging or closing)
  let processingMRs = $state<Record<number, "merging" | "closing" | null>>({});

  // Context menu state
  let contextMenu = $state<{ x: number; y: number; link: string } | null>(null);

  function handleContextMenu(e: MouseEvent, link: string) {
    if (!link) return;
    e.preventDefault();
    contextMenu = {
      x: e.clientX,
      y: e.clientY,
      link
    };
  }

  function closeContextMenu() {
    contextMenu = null;
  }

  onMount(() => {
    viewMode = localStorage.getItem("gittar_view_mode_mrs") || "list";
    searchQuery = localStorage.getItem("gittar_filter_mrs_search") || "";
    groupFilter = localStorage.getItem("gittar_filter_mrs_group") || "all";
    projectFilter = localStorage.getItem("gittar_filter_mrs_project") || "all";
    userFilter = localStorage.getItem("gittar_filter_mrs_user") || "all";
    activeMRTab = (localStorage.getItem("gittar_filter_mrs_tab") || "all") as any;

    window.addEventListener("click", closeContextMenu);
    window.addEventListener("contextmenu", closeContextMenu);
  });

  onDestroy(() => {
    window.removeEventListener("click", closeContextMenu);
    window.removeEventListener("contextmenu", closeContextMenu);
  });

  $effect(() => {
    localStorage.setItem("gittar_view_mode_mrs", viewMode);
  });

  $effect(() => {
    localStorage.setItem("gittar_filter_mrs_search", searchQuery);
    localStorage.setItem("gittar_filter_mrs_group", groupFilter);
    localStorage.setItem("gittar_filter_mrs_project", projectFilter);
    localStorage.setItem("gittar_filter_mrs_user", userFilter);
    localStorage.setItem("gittar_filter_mrs_tab", activeMRTab);
  });

  async function handleMerge(projectId: number, mrIID: number, mrId: number) {
    if (processingMRs[mrId]) return;
    if (!confirm("Are you sure you want to merge this Merge Request?")) {
      return;
    }
    processingMRs[mrId] = "merging";
    try {
      await MergeMergeRequest(projectId, mrIID);
      if (onRefresh) onRefresh();
    } catch (e: any) {
      console.error("Failed to merge MR:", e);
      alert("Failed to merge Merge Request: " + e.message);
    } finally {
      processingMRs[mrId] = null;
    }
  }

  async function handleClose(projectId: number, mrIID: number, mrId: number) {
    if (processingMRs[mrId]) return;
    if (!confirm("Are you sure you want to close this Merge Request?")) {
      return;
    }
    processingMRs[mrId] = "closing";
    try {
      await CloseMergeRequest(projectId, mrIID);
      if (onRefresh) onRefresh();
    } catch (e: any) {
      console.error("Failed to close MR:", e);
      alert("Failed to close Merge Request: " + e.message);
    } finally {
      processingMRs[mrId] = null;
    }
  }

  function getProjectPath(webUrl: string): string {
    if (!webUrl) return "";
    const parts = webUrl.split("/-/merge_requests/");
    if (parts.length > 0) {
      const urlParts = parts[0].split("/");
      if (urlParts.length > 3) {
        return urlParts.slice(3).join("/");
      }
    }
    return "";
  }

  // Dynamic filtering
  const assignedMRs = $derived(
    mergeRequests.filter((mr) =>
      (mr.assignees || []).some((a) => a.username === username)
    )
  );

  const authoredMRs = $derived(
    mergeRequests.filter((mr) => mr.author?.username === username)
  );

  const reviewRequests = $derived(
    mergeRequests.filter((mr) =>
      (mr.reviewers || []).some((r) => r.username === username)
    )
  );

  const uniqueGroups = $derived([
    ...new Set(mergeRequests.map((mr) => getProjectPath(mr.web_url).split("/")[0]).filter(Boolean)),
  ]);

  const uniqueProjects = $derived([
    ...new Set(mergeRequests.map((mr) => getProjectPath(mr.web_url)).filter(Boolean)),
  ]);

  const uniqueUsers = $derived([
    ...new Set([
      ...mergeRequests.map((mr) => mr.author?.name),
      ...mergeRequests.flatMap((mr) => (mr.assignees || []).map((a) => a.name)),
      ...mergeRequests.flatMap((mr) => (mr.reviewers || []).map((r) => r.name)),
    ].filter(Boolean)),
  ]);

  const baseList = $derived.by(() => {
    switch (activeMRTab) {
      case "all":
        return mergeRequests;
      case "assigned":
        return assignedMRs;
      case "authored":
        return authoredMRs;
      case "review":
        return reviewRequests;
    }
  });

  const filteredMRs = $derived(
    baseList.filter((mr) => {
      const projPath = getProjectPath(mr.web_url);

      const matchesSearch =
        searchQuery === "" ||
        (mr.title || "").toLowerCase().includes(searchQuery.toLowerCase()) ||
        (mr.source_branch || "").toLowerCase().includes(searchQuery.toLowerCase()) ||
        (mr.target_branch || "").toLowerCase().includes(searchQuery.toLowerCase()) ||
        projPath.toLowerCase().includes(searchQuery.toLowerCase());

      const matchesGroup =
        groupFilter === "all" || projPath.split("/")[0] === groupFilter;

      const matchesProject =
        projectFilter === "all" || projPath === projectFilter;

      const matchesUser =
        userFilter === "all" ||
        mr.author?.name === userFilter ||
        (mr.assignees || []).some((a) => a.name === userFilter) ||
        (mr.reviewers || []).some((r) => r.name === userFilter);

      return matchesSearch && matchesGroup && matchesProject && matchesUser;
    })
  );

  // Kanban Derived Columns
  const draftMRs = $derived(filteredMRs.filter((mr) => (mr.work_in_progress || mr.draft) && mr.state === "opened"));
  const reviewingMRs = $derived(
    filteredMRs.filter(
      (mr) =>
        !(mr.work_in_progress || mr.draft) &&
        mr.state === "opened" &&
        (mr.reviewers || []).some((r) => r.username === username)
    )
  );
  const inProgressMRs = $derived(
    filteredMRs.filter(
      (mr) =>
        !(mr.work_in_progress || mr.draft) &&
        mr.state === "opened" &&
        !(mr.reviewers || []).some((r) => r.username === username)
    )
  );
  const mergedMRs = $derived(filteredMRs.filter((mr) => mr.state === "merged"));
  const closedMRs = $derived(filteredMRs.filter((mr) => mr.state === "closed"));

  function resetFilters() {
    searchQuery = "";
    groupFilter = "all";
    projectFilter = "all";
    userFilter = "all";
  }

  function getLabelColorHash(label: string): string {
    let hash = 0;
    for (let i = 0; i < label.length; i++) {
      hash = label.charCodeAt(i) + ((hash << 5) - hash);
    }
    const h = Math.abs(hash % 360);
    return `hsl(${h}, 70%, 65%)`;
  }

  function getPipelineStatusClasses(status: string): string {
    switch (status?.toLowerCase()) {
      case "success":
        return "bg-emerald-500/10 text-emerald-400 border-emerald-500/30 hover:border-emerald-500/50";
      case "failed":
        return "bg-rose-500/10 text-rose-400 border-rose-500/30 hover:border-rose-500/50";
      case "running":
      case "pending":
        return "bg-amber-500/10 text-amber-400 border-amber-500/30 hover:border-amber-500/50";
      case "canceled":
      case "skipped":
        return "bg-slate-500/10 text-slate-400 border-slate-500/30 hover:border-slate-500/50";
      default:
        return "bg-slate-700/10 text-slate-400 border-slate-700/30 hover:border-slate-700/50";
    }
  }

  function formatRelativeTime(dateStr: any): string {
    if (!dateStr) return "";
    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMins / 60);
    const diffDays = Math.floor(diffHours / 24);

    if (diffMins < 1) return "just now";
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    if (diffDays === 1) return "yesterday";
    return `${diffDays}d ago`;
  }

  function getBorderLeftAccent(mr: MergeRequest): string {
    if (mr.state === "merged") return "border-l-indigo-500/40";
    if (mr.state === "closed") return "border-l-slate-700/30";
    if (mr.work_in_progress || mr.draft) return "border-l-slate-600/40";
    if (mr.head_pipeline) {
      switch (mr.head_pipeline.status?.toLowerCase()) {
        case "success":
          return "border-l-emerald-500/50";
        case "failed":
          return "border-l-rose-500/50";
        case "running":
        case "pending":
          return "border-l-amber-500/50";
      }
    }
    return "border-l-slate-800/60";
  }

  // Drag Handlers
  function handleDragStart(e: DragEvent, mr: MergeRequest) {
    draggedMR = mr;
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = "move";
      e.dataTransfer.setData("text/plain", mr.id.toString());
    }
  }

  function handleDragEnd() {
    draggedMR = null;
    isDraggingOverMerge = false;
    isDraggingOverClose = false;
  }

  function handleDragOverMerge(e: DragEvent) {
    e.preventDefault();
    isDraggingOverMerge = true;
  }

  function handleDragLeaveMerge() {
    isDraggingOverMerge = false;
  }

  async function handleDropMerge(e: DragEvent) {
    e.preventDefault();
    isDraggingOverMerge = false;
    if (draggedMR) {
      const mr = draggedMR;
      if (mr.state === "opened") {
        await handleMerge(mr.project_id, mr.iid, mr.id);
      }
      draggedMR = null;
    }
  }

  function handleDragOverClose(e: DragEvent) {
    e.preventDefault();
    isDraggingOverClose = true;
  }

  function handleDragLeaveClose() {
    isDraggingOverClose = false;
  }

  async function handleDropClose(e: DragEvent) {
    e.preventDefault();
    isDraggingOverClose = false;
    if (draggedMR) {
      const mr = draggedMR;
      if (mr.state === "opened") {
        await handleClose(mr.project_id, mr.iid, mr.id);
      }
      draggedMR = null;
    }
  }
</script>

<div class="h-full flex flex-col">
  <!-- Panel Header -->
  <div class="p-6 border-b border-slate-900/60 flex items-center justify-between">
    <div>
      <h2 class="text-xl font-semibold text-white">MR Gatekeeper</h2>
      <p class="text-slate-400 text-xs mt-1">Aggregate merge requests assigned to, reviewed by, or authored by you.</p>
    </div>

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

  <!-- Sub-tab Navigation -->
  <div class="px-6 py-3 border-b border-slate-900/40 bg-slate-950/20 flex space-x-1.5">
    <button
      onclick={() => (activeMRTab = "all")}
      class="px-3 py-1.5 text-xs font-semibold rounded-lg transition-colors flex items-center space-x-2 {activeMRTab === 'all' ? 'bg-slate-900 text-white border border-slate-800' : 'text-slate-400 hover:text-slate-200'}"
    >
      <span>All</span>
      {#if mergeRequests.length > 0}
        <span class="px-1.5 py-0.5 text-[10px] bg-slate-800 text-slate-355 rounded-full">
          {mergeRequests.length}
        </span>
      {/if}
    </button>

    <button
      onclick={() => (activeMRTab = "assigned")}
      class="px-3 py-1.5 text-xs font-semibold rounded-lg transition-colors flex items-center space-x-2 {activeMRTab === 'assigned' ? 'bg-slate-900 text-white border border-slate-800' : 'text-slate-400 hover:text-slate-200'}"
    >
      <span>Assigned to Me</span>
      {#if assignedMRs.length > 0}
        <span class="px-1.5 py-0.5 text-[10px] bg-indigo-500/10 text-indigo-400 rounded-full">
          {assignedMRs.length}
        </span>
      {/if}
    </button>

    <button
      onclick={() => (activeMRTab = "authored")}
      class="px-3 py-1.5 text-xs font-semibold rounded-lg transition-colors flex items-center space-x-2 {activeMRTab === 'authored' ? 'bg-slate-900 text-white border border-slate-800' : 'text-slate-400 hover:text-slate-200'}"
    >
      <span>Authored by Me</span>
      {#if authoredMRs.length > 0}
        <span class="px-1.5 py-0.5 text-[10px] bg-slate-800 text-slate-355 rounded-full">
          {authoredMRs.length}
        </span>
      {/if}
    </button>

    <button
      onclick={() => (activeMRTab = "review")}
      class="px-3 py-1.5 text-xs font-semibold rounded-lg transition-colors flex items-center space-x-2 {activeMRTab === 'review' ? 'bg-slate-900 text-white border border-slate-800' : 'text-slate-400 hover:text-slate-200'}"
    >
      <span>Review Requests</span>
      {#if reviewRequests.length > 0}
        <span class="px-1.5 py-0.5 text-[10px] bg-emerald-500/10 text-emerald-400 rounded-full">
          {reviewRequests.length}
        </span>
      {/if}
    </button>
  </div>

  <!-- Filter & Search Toolbar -->
  {#if mergeRequests.length > 0}
    <div class="px-6 py-3 border-b border-slate-900/40 bg-slate-950/20 flex flex-wrap items-center gap-3 select-none">
      <!-- Search Input -->
      <div class="relative w-48">
        <input
          type="text"
          bind:value={searchQuery}
          placeholder="Search MRs..."
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
        {#each uniqueProjects as proj (proj)}
          <option value={proj}>{proj}</option>
        {/each}
      </select>

      <!-- User Filter -->
      <select
        bind:value={userFilter}
        class="px-2 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-xs text-slate-300 outline-none cursor-pointer focus:border-indigo-650"
      >
        <option value="all">All Users</option>
        {#each uniqueUsers as usr (usr)}
          <option value={usr}>{usr}</option>
        {/each}
      </select>

      <!-- Reset Filters Button -->
      {#if searchQuery !== "" || groupFilter !== "all" || projectFilter !== "all" || userFilter !== "all"}
        <button
          onclick={resetFilters}
          class="px-3 py-1.5 border border-indigo-500/30 hover:border-indigo-550/40 bg-indigo-550/10 hover:bg-indigo-550/20 text-indigo-400 text-xs font-semibold rounded-lg transition"
        >
          Reset Filters
        </button>
      {/if}
    </div>
  {/if}

  <!-- Content Area -->
  <div class="flex-1 overflow-hidden relative">
    {#if baseList.length === 0}
      <div class="h-[75%] flex flex-col items-center justify-center text-center">
        <div class="w-12 h-12 rounded-full bg-slate-950/40 border border-slate-900 flex items-center justify-center text-slate-500 mb-4">
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
          </svg>
        </div>
        <h3 class="text-base font-semibold text-slate-200">No Merge Requests</h3>
        <p class="text-slate-500 text-sm mt-1 max-w-[280px]">No open merge requests found matching this filter.</p>
      </div>
    {:else if filteredMRs.length === 0 && viewMode === "list"}
      <!-- Empty Filter State -->
      <div class="h-[70%] flex flex-col items-center justify-center text-center">
        <div class="w-12 h-12 rounded-full bg-slate-950/40 border border-slate-900 flex items-center justify-center text-slate-500 mb-4">
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
          </svg>
        </div>
        <h3 class="text-base font-semibold text-slate-350">No Results Found</h3>
        <p class="text-slate-500 text-sm mt-1 max-w-[280px]">No merge requests match your filter criteria. Try resetting or adjusting your search.</p>
        <button
          onclick={resetFilters}
          class="mt-4 px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-lg transition"
        >
          Reset Filters
        </button>
      </div>
    {:else if viewMode === "list"}
      <div class="h-full overflow-y-auto p-6 space-y-3">
        {#each filteredMRs as mr (mr.id)}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div
            oncontextmenu={(e) => handleContextMenu(e, mr.web_url)}
            class="group p-4 bg-slate-950/20 border-t border-r border-b border-l-2 border-y-slate-900/40 border-r-slate-900/40 {getBorderLeftAccent(mr)} hover:bg-slate-900/25 hover:border-y-slate-800/60 hover:border-r-slate-800/60 hover:shadow-md hover:shadow-indigo-500/5 rounded-xl transition-all duration-200 flex items-start justify-between relative"
          >
            <div class="min-w-0 flex-1 pr-4">
              <!-- Title, ID, State & Draft Indicator -->
              <div class="flex items-center space-x-2.5 flex-wrap gap-y-1.5">
                {#if mr.work_in_progress || mr.draft}
                  <span class="px-1.5 py-0.5 text-[9px] font-extrabold tracking-wider bg-slate-850 text-slate-400 border border-slate-700/60 rounded">
                    DRAFT
                  </span>
                {/if}
                
                {#if mr.state === "merged"}
                  <span class="px-1.5 py-0.5 text-[9px] font-extrabold tracking-wider bg-indigo-500/10 text-indigo-400 border border-indigo-500/30 rounded">
                    MERGED
                  </span>
                {:else if mr.state === "closed"}
                  <span class="px-1.5 py-0.5 text-[9px] font-extrabold tracking-wider bg-slate-500/10 text-slate-400 border border-slate-500/30 rounded">
                    CLOSED
                  </span>
                {/if}

                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <span
                  onclick={() => Browser.OpenURL(mr.web_url)}
                  class="text-sm font-semibold text-slate-100 hover:text-indigo-400 transition cursor-pointer"
                  title={mr.title}
                >
                  {mr.title}
                </span>
                
                <span class="text-xs font-mono text-slate-500">
                  #{mr.iid}
                </span>
              </div>

              <!-- Project and Author Info, relative created / updated time -->
              <div class="flex items-center space-x-2 flex-wrap text-xs text-slate-400 mt-2">
                <span class="text-indigo-400 font-semibold truncate max-w-[200px]" title="Project">
                  {getProjectPath(mr.web_url)}
                </span>
                <span class="text-slate-750">•</span>
                <span class="flex items-center space-x-1.5">
                  {#if mr.author?.avatar_url}
                    <img src={mr.author.avatar_url} alt={mr.author.name} class="w-4.5 h-4.5 rounded-full border border-slate-800" />
                  {/if}
                  <span class="text-slate-300 font-medium">{mr.author?.name}</span>
                </span>
                <span class="text-slate-750">•</span>
                <span class="text-slate-500">created {formatRelativeTime(mr.created_at)}</span>
                <span class="text-slate-750">•</span>
                <span class="text-slate-500">updated {formatRelativeTime(mr.updated_at)}</span>
              </div>

              <!-- Branches Info (GitFlow representation) -->
              <div class="flex items-center space-x-2 text-xs text-slate-400 mt-2.5 font-mono">
                <span class="bg-slate-950/70 border border-slate-900 px-1.5 py-0.5 rounded text-[11px] text-slate-355">
                  {mr.source_branch}
                </span>
                <span class="text-slate-650 flex items-center">
                  <!-- Branch Merge Icon -->
                  <svg class="w-3.5 h-3.5 mx-0.5 text-slate-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M8 7a3 3 0 100-6 3 3 0 000 6zm0 10a3 3 0 100-6 3 3 0 000 6zm0-10v4m0 0l4-4m-4 4L4 7m4 4h4" />
                  </svg>
                </span>
                <span class="bg-slate-950/70 border border-slate-900 px-1.5 py-0.5 rounded text-[11px] text-slate-355">
                  {mr.target_branch}
                </span>
              </div>

              <!-- Labels (dynamic color) -->
              {#if mr.labels && mr.labels.length > 0}
                <div class="flex flex-wrap gap-1.5 mt-3">
                  {#each mr.labels as label (label)}
                    <span
                      class="px-2 py-0.5 text-[10px] font-semibold rounded-md border"
                      style="background-color: {getLabelColorHash(label).replace('hsl', 'hsla').replace(')', ', 0.08)')}; color: {getLabelColorHash(label)}; border-color: {getLabelColorHash(label).replace('hsl', 'hsla').replace(')', ', 0.25)')}"
                    >
                      {label}
                    </span>
                  {/each}
                </div>
              {/if}
            </div>

            <!-- Right section: Pipelines, notes, assignees, reviewers, actions -->
            <div class="flex items-center space-x-4 shrink-0 pl-4 self-center">
              <!-- Comments / notes count -->
              {#if mr.user_notes_count > 0}
                <div class="flex items-center space-x-1 text-xs text-slate-400 bg-slate-950/40 border border-slate-900/80 px-2 py-1 rounded-lg" title="Comments">
                  <svg class="w-3.5 h-3.5 text-slate-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
                  </svg>
                  <span>{mr.user_notes_count}</span>
                </div>
              {/if}

              <!-- Pipeline status badge -->
              {#if mr.head_pipeline && mr.head_pipeline.id > 0}
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <!-- svelte-ignore a11y_no_static_element_interactions -->
                <div
                  onclick={(e) => { e.stopPropagation(); if (mr.head_pipeline?.web_url) Browser.OpenURL(mr.head_pipeline.web_url); }}
                  oncontextmenu={(e) => { e.stopPropagation(); if (mr.head_pipeline?.web_url) handleContextMenu(e, mr.head_pipeline.web_url); }}
                  class="px-2 py-1 border text-[11px] font-bold uppercase tracking-wider rounded-lg flex items-center space-x-1.5 cursor-pointer transition select-none {getPipelineStatusClasses(mr.head_pipeline?.status || '')}"
                  title="Head Pipeline: {mr.head_pipeline?.status || ''} (Click to open, right-click to copy)"
                >
                  {#if mr.head_pipeline.status === "success"}
                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                  {:else if mr.head_pipeline.status === "failed"}
                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                  {:else if mr.head_pipeline.status === "running" || mr.head_pipeline.status === "pending"}
                    <div class="w-3 h-3 border-2 border-amber-500 border-t-transparent rounded-full animate-spin"></div>
                  {:else}
                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                  {/if}
                  <span>Pipeline</span>
                </div>
              {/if}

              <!-- Assignees & Reviewers Avatars -->
              {#if (mr.assignees && mr.assignees.length > 0) || (mr.reviewers && mr.reviewers.length > 0)}
                <div class="flex items-center -space-x-1.5">
                  {#each mr.assignees || [] as assignee (assignee.id || assignee.username)}
                    {#if assignee.avatar_url}
                      <img
                        src={assignee.avatar_url}
                        alt="Assignee: {assignee.name}"
                        class="w-6 h-6 rounded-full border border-slate-955 object-cover ring-1 ring-slate-800/80"
                        title="Assignee: {assignee.name}"
                      />
                    {/if}
                  {/each}
                  {#each mr.reviewers || [] as reviewer (reviewer.id || reviewer.username)}
                    {#if reviewer.avatar_url}
                      <img
                        src={reviewer.avatar_url}
                        alt="Reviewer: {reviewer.name}"
                        class="w-6 h-6 rounded-full border border-slate-955 object-cover ring-1 ring-slate-800/50 filter brightness-90"
                        title="Reviewer: {reviewer.name}"
                      />
                    {/if}
                  {/each}
                </div>
              {/if}

              <!-- Merge & Close action controls -->
              {#if mr.state === "opened"}
                <div class="flex items-center space-x-1.5 pl-2">
                  <button
                    onclick={(e) => { e.stopPropagation(); handleMerge(mr.project_id, mr.iid, mr.id); }}
                    disabled={!!processingMRs[mr.id]}
                    class="px-2.5 py-1.5 bg-emerald-600/10 hover:bg-emerald-600 text-emerald-400 hover:text-white border border-emerald-500/30 hover:border-emerald-500 font-semibold text-xs rounded-lg transition disabled:opacity-40 flex items-center space-x-1 shrink-0"
                  >
                    {#if processingMRs[mr.id] === "merging"}
                      <div class="w-3 h-3 border border-emerald-400 border-t-transparent rounded-full animate-spin"></div>
                      <span>Merging...</span>
                    {:else}
                      <span>Merge</span>
                    {/if}
                  </button>

                  <button
                    onclick={(e) => { e.stopPropagation(); handleClose(mr.project_id, mr.iid, mr.id); }}
                    disabled={!!processingMRs[mr.id]}
                    class="px-2.5 py-1.5 bg-rose-600/10 hover:bg-rose-600 text-rose-400 hover:text-white border border-rose-500/30 hover:border-rose-500 font-semibold text-xs rounded-lg transition disabled:opacity-40 flex items-center space-x-1 shrink-0"
                  >
                    {#if processingMRs[mr.id] === "closing"}
                      <div class="w-3 h-3 border border-rose-450 border-t-transparent rounded-full animate-spin"></div>
                      <span>Closing...</span>
                    {:else}
                      <span>Close</span>
                    {/if}
                  </button>
                </div>
              {/if}

              <!-- Open in Browser Button -->
              <button
                onclick={(e) => { e.stopPropagation(); Browser.OpenURL(mr.web_url); }}
                class="px-3 py-1.5 border border-slate-800 hover:border-slate-700 bg-slate-950/44 rounded-lg text-slate-400 hover:text-slate-200 transition shrink-0 self-center"
                title="Open Merge Request in Browser"
              >
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                </svg>
              </button>
            </div>
          </div>
        {/each}
      </div>
    {:else}
      <!-- Kanban Board View -->
      <div class="h-full w-full overflow-x-auto p-6 flex space-x-4 select-none">
        
        <!-- Column 1: Drafts -->
        <div class="flex-1 min-w-[280px] max-w-[350px] bg-slate-900/15 border border-slate-900/80 rounded-2xl p-3.5 flex flex-col h-full">
          <div class="flex items-center justify-between pb-2 mb-3 border-b border-slate-900/50 shrink-0">
            <h3 class="text-xs font-bold uppercase tracking-wider text-slate-400">Drafts</h3>
            <span class="px-1.5 py-0.5 text-[10px] font-bold bg-slate-800 text-slate-400 rounded-full">{draftMRs.length}</span>
          </div>
          <div class="flex-1 overflow-y-auto space-y-2.5 pr-1">
            {#each draftMRs as mr (mr.id)}
              <!-- svelte-ignore a11y_no_static_element_interactions -->
              <div
                draggable="true"
                ondragstart={(e) => handleDragStart(e, mr)}
                ondragend={handleDragEnd}
                class="group p-3 bg-slate-950/35 hover:bg-slate-950/60 border border-slate-900/60 hover:border-slate-800/70 rounded-xl cursor-grab active:cursor-grabbing transition"
              >
                <div class="text-xs font-semibold text-indigo-400 truncate">{getProjectPath(mr.web_url)}</div>
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
                <h4 onclick={() => Browser.OpenURL(mr.web_url)} class="text-sm font-semibold text-slate-100 hover:text-indigo-400 cursor-pointer mt-1 line-clamp-2">{mr.title}</h4>
                <div class="flex items-center justify-between mt-3 text-[10px] text-slate-500">
                  <span>#{mr.iid}</span>
                  <span>{formatRelativeTime(mr.updated_at)}</span>
                </div>
              </div>
            {/each}
          </div>
        </div>

        <!-- Column 2: In Progress -->
        <div class="flex-1 min-w-[280px] max-w-[350px] bg-slate-900/15 border border-slate-900/80 rounded-2xl p-3.5 flex flex-col h-full">
          <div class="flex items-center justify-between pb-2 mb-3 border-b border-slate-900/50 shrink-0">
            <h3 class="text-xs font-bold uppercase tracking-wider text-blue-400">In Progress</h3>
            <span class="px-1.5 py-0.5 text-[10px] font-bold bg-blue-500/10 text-blue-400 rounded-full">{inProgressMRs.length}</span>
          </div>
          <div class="flex-1 overflow-y-auto space-y-2.5 pr-1">
            {#each inProgressMRs as mr (mr.id)}
              <!-- svelte-ignore a11y_no_static_element_interactions -->
              <div
                draggable="true"
                ondragstart={(e) => handleDragStart(e, mr)}
                ondragend={handleDragEnd}
                class="group p-3 bg-slate-950/35 hover:bg-slate-950/60 border border-slate-900/60 hover:border-slate-800/70 rounded-xl cursor-grab active:cursor-grabbing transition"
              >
                <div class="text-xs font-semibold text-indigo-400 truncate">{getProjectPath(mr.web_url)}</div>
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
                <h4 onclick={() => Browser.OpenURL(mr.web_url)} class="text-sm font-semibold text-slate-100 hover:text-indigo-400 cursor-pointer mt-1 line-clamp-2">{mr.title}</h4>
                <div class="flex items-center justify-between mt-3 text-[10px] text-slate-500">
                  <span>#{mr.iid}</span>
                  <span>{formatRelativeTime(mr.updated_at)}</span>
                </div>
              </div>
            {/each}
          </div>
        </div>

        <!-- Column 3: Reviewing -->
        <div class="flex-1 min-w-[280px] max-w-[350px] bg-slate-900/15 border border-slate-900/80 rounded-2xl p-3.5 flex flex-col h-full">
          <div class="flex items-center justify-between pb-2 mb-3 border-b border-slate-900/50 shrink-0">
            <h3 class="text-xs font-bold uppercase tracking-wider text-purple-400">Reviewing</h3>
            <span class="px-1.5 py-0.5 text-[10px] font-bold bg-purple-500/10 text-purple-400 rounded-full">{reviewingMRs.length}</span>
          </div>
          <div class="flex-1 overflow-y-auto space-y-2.5 pr-1">
            {#each reviewingMRs as mr (mr.id)}
              <!-- svelte-ignore a11y_no_static_element_interactions -->
              <div
                draggable="true"
                ondragstart={(e) => handleDragStart(e, mr)}
                ondragend={handleDragEnd}
                class="group p-3 bg-slate-950/35 hover:bg-slate-950/60 border border-slate-900/60 hover:border-slate-800/70 rounded-xl cursor-grab active:cursor-grabbing transition"
              >
                <div class="text-xs font-semibold text-indigo-400 truncate">{getProjectPath(mr.web_url)}</div>
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
                <h4 onclick={() => Browser.OpenURL(mr.web_url)} class="text-sm font-semibold text-slate-100 hover:text-indigo-400 cursor-pointer mt-1 line-clamp-2">{mr.title}</h4>
                <div class="flex items-center justify-between mt-3 text-[10px] text-slate-500">
                  <span>#{mr.iid}</span>
                  <span>{formatRelativeTime(mr.updated_at)}</span>
                </div>
              </div>
            {/each}
          </div>
        </div>

        <!-- Column 4: Merged Drop Zone / Recently Merged -->
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div
          ondragover={handleDragOverMerge}
          ondragleave={handleDragLeaveMerge}
          ondrop={handleDropMerge}
          class="flex-1 min-w-[280px] max-w-[350px] rounded-2xl p-3.5 flex flex-col h-full border-2 transition-all duration-200 {isDraggingOverMerge ? 'bg-emerald-950/10 border-emerald-500/80' : draggedMR ? 'bg-slate-900/10 border-dashed border-emerald-500/30' : 'bg-slate-900/15 border-slate-900/80'}"
        >
          <div class="flex items-center justify-between pb-2 mb-3 border-b border-slate-900/50 shrink-0">
            <h3 class="text-xs font-bold uppercase tracking-wider text-emerald-400">Merged</h3>
            <span class="px-1.5 py-0.5 text-[10px] font-bold bg-emerald-500/10 text-emerald-400 rounded-full">{mergedMRs.length}</span>
          </div>
          <div class="flex-1 overflow-y-auto space-y-2.5 pr-1">
            {#if draggedMR && mergedMRs.length === 0}
              <div class="h-24 flex flex-col items-center justify-center text-center text-slate-550 border border-dashed border-emerald-500/20 rounded-xl bg-emerald-500/[0.02] p-4 pointer-events-none">
                <svg class="w-5 h-5 text-emerald-500 animate-pulse mb-1.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <div class="text-[10px] font-bold">Drop here to Merge MR</div>
              </div>
            {/if}

            {#each mergedMRs as mr (mr.id)}
              <div class="group p-3 bg-slate-950/15 border border-slate-900/40 rounded-xl opacity-60 hover:opacity-95 transition duration-150">
                <div class="text-xs font-semibold text-indigo-400 truncate">{getProjectPath(mr.web_url)}</div>
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
                <h4 onclick={() => Browser.OpenURL(mr.web_url)} class="text-sm font-semibold text-slate-200 hover:text-indigo-400 cursor-pointer mt-1 line-clamp-2">{mr.title}</h4>
                <div class="flex items-center justify-between mt-3 text-[10px] text-slate-500">
                  <span>#{mr.iid}</span>
                  <span>merged {formatRelativeTime(mr.merged_at || mr.updated_at)}</span>
                </div>
              </div>
            {/each}
          </div>
        </div>

        <!-- Column 5: Closed Drop Zone / Recently Closed -->
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div
          ondragover={handleDragOverClose}
          ondragleave={handleDragLeaveClose}
          ondrop={handleDropClose}
          class="flex-1 min-w-[280px] max-w-[350px] rounded-2xl p-3.5 flex flex-col h-full border-2 transition-all duration-200 {isDraggingOverClose ? 'bg-rose-955/10 border-rose-500/80' : draggedMR ? 'bg-slate-900/10 border-dashed border-rose-500/30' : 'bg-slate-900/15 border-slate-900/80'}"
        >
          <div class="flex items-center justify-between pb-2 mb-3 border-b border-slate-900/50 shrink-0">
            <h3 class="text-xs font-bold uppercase tracking-wider text-rose-450">Closed</h3>
            <span class="px-1.5 py-0.5 text-[10px] font-bold bg-rose-500/10 text-rose-400 rounded-full">{closedMRs.length}</span>
          </div>
          <div class="flex-1 overflow-y-auto space-y-2.5 pr-1">
            {#if draggedMR && closedMRs.length === 0}
              <div class="h-24 flex flex-col items-center justify-center text-center text-slate-555 border border-dashed border-rose-500/20 rounded-xl bg-rose-500/[0.02] p-4 pointer-events-none">
                <svg class="w-5 h-5 text-rose-550 animate-pulse mb-1.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <div class="text-[10px] font-bold">Drop here to Close MR</div>
              </div>
            {/if}

            {#each closedMRs as mr (mr.id)}
              <div class="group p-3 bg-slate-950/15 border border-slate-900/40 rounded-xl opacity-60 hover:opacity-95 transition duration-150">
                <div class="text-xs font-semibold text-indigo-400 truncate">{getProjectPath(mr.web_url)}</div>
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
                <h4 onclick={() => Browser.OpenURL(mr.web_url)} class="text-sm font-semibold text-slate-200 hover:text-indigo-400 cursor-pointer mt-1 line-clamp-2">{mr.title}</h4>
                <div class="flex items-center justify-between mt-3 text-[10px] text-slate-500">
                  <span>#{mr.iid}</span>
                  <span>closed {formatRelativeTime(mr.closed_at || mr.updated_at)}</span>
                </div>
              </div>
            {/each}
          </div>
        </div>

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
