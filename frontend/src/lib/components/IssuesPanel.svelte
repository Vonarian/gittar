<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import type { Issue } from "../../../bindings/gittar/internal/gitlab/models";
  import { Browser, Clipboard } from "@wailsio/runtime";
  import { CloseIssue } from "../../../bindings/gittar/internal/service/appservice";

  interface Props {
    issues: Issue[];
    username: string;
    onRefresh?: () => void;
  }

  let { issues = [], username, onRefresh }: Props = $props();

  let activeIssueTab = $state<"all" | "assigned" | "authored">("all");
  let viewMode = $state("list");

  // Search & Filter state variables
  let searchQuery = $state("");
  let groupFilter = $state("all");
  let projectFilter = $state("all");
  let userFilter = $state("all");
  let involvementFilter = $state("all");

  // Drag-and-drop state
  let draggedIssue = $state<Issue | null>(null);
  let isDraggingOverClose = $state(false);

  // Local state for processing actions (e.g. closing)
  let processingIssues = $state<Record<number, "closing" | null>>({});

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
    viewMode = localStorage.getItem("gittar_view_mode_issues") || "list";
    searchQuery = localStorage.getItem("gittar_filter_issues_search") || "";
    groupFilter = localStorage.getItem("gittar_filter_issues_group") || "all";
    projectFilter = localStorage.getItem("gittar_filter_issues_project") || "all";
    userFilter = localStorage.getItem("gittar_filter_issues_user") || "all";
    involvementFilter = localStorage.getItem("gittar_filter_issues_involvement") || "all";
    activeIssueTab = (localStorage.getItem("gittar_filter_issues_tab") || "all") as any;

    window.addEventListener("click", closeContextMenu);
    window.addEventListener("contextmenu", closeContextMenu);
  });

  onDestroy(() => {
    window.removeEventListener("click", closeContextMenu);
    window.removeEventListener("contextmenu", closeContextMenu);
  });

  $effect(() => {
    localStorage.setItem("gittar_view_mode_issues", viewMode);
  });

  $effect(() => {
    localStorage.setItem("gittar_filter_issues_search", searchQuery);
    localStorage.setItem("gittar_filter_issues_group", groupFilter);
    localStorage.setItem("gittar_filter_issues_project", projectFilter);
    localStorage.setItem("gittar_filter_issues_user", userFilter);
    localStorage.setItem("gittar_filter_issues_involvement", involvementFilter);
    localStorage.setItem("gittar_filter_issues_tab", activeIssueTab);
  });

  async function handleClose(projectId: number, issueIID: number, issueId: number) {
    if (processingIssues[issueId]) return;
    if (!confirm("Are you sure you want to close this Issue?")) {
      return;
    }
    processingIssues[issueId] = "closing";
    try {
      await CloseIssue(projectId, issueIID);
      if (onRefresh) onRefresh();
    } catch (e: any) {
      console.error("Failed to close Issue:", e);
      alert("Failed to close Issue: " + e.message);
    } finally {
      processingIssues[issueId] = null;
    }
  }

  function getProjectPath(webUrl: string): string {
    if (!webUrl) return "";
    const parts = webUrl.split("/-/issues/");
    if (parts.length > 0) {
      const urlParts = parts[0].split("/");
      if (urlParts.length > 3) {
        return urlParts.slice(3).join("/");
      }
    }
    return "";
  }

  // Dynamic filtering
  const assignedIssues = $derived(
    issues.filter((issue) =>
      (issue.assignees || []).some((a) => a.username === username)
    )
  );

  const authoredIssues = $derived(
    issues.filter((issue) => issue.author?.username === username)
  );

  const uniqueGroups = $derived([
    ...new Set(issues.map((issue) => getProjectPath(issue.web_url).split("/")[0]).filter(Boolean)),
  ]);

  const uniqueProjects = $derived([
    ...new Set(issues.map((issue) => getProjectPath(issue.web_url)).filter(Boolean)),
  ]);

  const uniqueUsers = $derived([
    ...new Set([
      ...issues.map((issue) => issue.author?.name),
      ...issues.flatMap((issue) => (issue.assignees || []).map((a) => a.name)),
    ].filter(Boolean)),
  ]);

  const baseList = $derived.by(() => {
    switch (activeIssueTab) {
      case "all":
        return issues;
      case "assigned":
        return assignedIssues;
      case "authored":
        return authoredIssues;
    }
  });

  const filteredIssues = $derived(
    baseList.filter((issue) => {
      const projPath = getProjectPath(issue.web_url);

      const matchesSearch =
        searchQuery === "" ||
        (issue.title || "").toLowerCase().includes(searchQuery.toLowerCase()) ||
        (issue.description || "").toLowerCase().includes(searchQuery.toLowerCase()) ||
        projPath.toLowerCase().includes(searchQuery.toLowerCase());

      const matchesGroup =
        groupFilter === "all" || projPath.split("/")[0] === groupFilter;

      const matchesProject =
        projectFilter === "all" || projPath === projectFilter;

      const matchesUser =
        userFilter === "all" ||
        issue.author?.name === userFilter ||
        (issue.assignees || []).some((a) => a.name === userFilter);

      const matchesInvolvement =
        involvementFilter === "all" ||
        issue.author?.username === username ||
        (issue.assignees || []).some((a) => a.username === username);

      return matchesSearch && matchesGroup && matchesProject && matchesUser && matchesInvolvement;
    })
  );

  // Kanban Columns
  // 1. Open / Backlog: State is open/opened and NO assignees
  const backlogIssues = $derived(
    filteredIssues.filter((i) => i.state === "opened" && (!i.assignees || i.assignees.length === 0))
  );
  // 2. In Progress: State is open/opened and has assignees
  const inProgressIssues = $derived(
    filteredIssues.filter((i) => i.state === "opened" && i.assignees && i.assignees.length > 0)
  );
  // 3. Closed
  const closedIssues = $derived(
    filteredIssues.filter((i) => i.state === "closed")
  );

  function resetFilters() {
    searchQuery = "";
    groupFilter = "all";
    projectFilter = "all";
    userFilter = "all";
    involvementFilter = "all";
  }

  function getLabelColorHash(label: string): string {
    let hash = 0;
    for (let i = 0; i < label.length; i++) {
      hash = label.charCodeAt(i) + ((hash << 5) - hash);
    }
    const h = Math.abs(hash % 360);
    return `hsl(${h}, 70%, 65%)`;
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

  function getBorderLeftAccent(issue: Issue): string {
    if (issue.state === "closed") return "border-l-slate-700/30";
    return "border-l-sky-500/50";
  }

  // Drag Handlers
  function handleDragStart(e: DragEvent, issue: Issue) {
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = "move";
      e.dataTransfer.setData("text/plain", issue.id.toString());
    }
    setTimeout(() => {
      draggedIssue = issue;
    }, 0);
  }

  function handleDragEnd() {
    draggedIssue = null;
    isDraggingOverClose = false;
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
    if (draggedIssue) {
      const issue = draggedIssue;
      if (issue.state === "opened") {
        await handleClose(issue.project_id, issue.iid, issue.id);
      }
      draggedIssue = null;
    }
  }
</script>

<div class="h-full flex flex-col">
  <!-- Panel Header -->
  <div class="p-6 border-b border-slate-900/60 flex items-center justify-between">
    <div>
      <h2 class="text-xl font-semibold text-white">Issues Tracker</h2>
      <p class="text-slate-400 text-xs mt-1">Manage issues assigned to you, authored by you, or active in watched repositories.</p>
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
      onclick={() => (activeIssueTab = "all")}
      class="px-3 py-1.5 text-xs font-semibold rounded-lg transition-colors flex items-center space-x-2 {activeIssueTab === 'all' ? 'bg-slate-900 text-white border border-slate-800' : 'text-slate-400 hover:text-slate-200'}"
    >
      <span>All</span>
      {#if issues.length > 0}
        <span class="px-1.5 py-0.5 text-[10px] bg-slate-800 text-slate-355 rounded-full">
          {issues.length}
        </span>
      {/if}
    </button>

    <button
      onclick={() => (activeIssueTab = "assigned")}
      class="px-3 py-1.5 text-xs font-semibold rounded-lg transition-colors flex items-center space-x-2 {activeIssueTab === 'assigned' ? 'bg-slate-900 text-white border border-slate-800' : 'text-slate-400 hover:text-slate-200'}"
    >
      <span>Assigned to Me</span>
      {#if assignedIssues.length > 0}
        <span class="px-1.5 py-0.5 text-[10px] bg-sky-500/10 text-sky-400 rounded-full">
          {assignedIssues.length}
        </span>
      {/if}
    </button>

    <button
      onclick={() => (activeIssueTab = "authored")}
      class="px-3 py-1.5 text-xs font-semibold rounded-lg transition-colors flex items-center space-x-2 {activeIssueTab === 'authored' ? 'bg-slate-900 text-white border border-slate-800' : 'text-slate-400 hover:text-slate-200'}"
    >
      <span>Authored by Me</span>
      {#if authoredIssues.length > 0}
        <span class="px-1.5 py-0.5 text-[10px] bg-slate-800 text-slate-355 rounded-full">
          {authoredIssues.length}
        </span>
      {/if}
    </button>
  </div>

  <!-- Filter & Search Toolbar -->
  {#if issues.length > 0}
    <div class="px-6 py-3 border-b border-slate-900/40 bg-slate-950/20 flex flex-wrap items-center gap-3 select-none">
      <!-- Search Input -->
      <div class="relative w-48">
        <input
          type="text"
          bind:value={searchQuery}
          placeholder="Search issues..."
          class="w-full px-2.5 py-1.5 bg-slate-950 border border-slate-800 focus:border-indigo-600 rounded-lg text-xs text-slate-200 outline-none transition"
        />
      </div>

      <!-- Group Filter -->
      <select
        bind:value={groupFilter}
        class="px-2 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-xs text-slate-300 outline-none cursor-pointer focus:border-indigo-650"
      >
        <option value="all">All Groups</option>
        {#each uniqueGroups as g}
          <option value={g}>{g}</option>
        {/each}
      </select>

      <!-- Project Filter -->
      <select
        bind:value={projectFilter}
        class="px-2 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-xs text-slate-300 outline-none cursor-pointer focus:border-indigo-650"
      >
        <option value="all">All Projects</option>
        {#each uniqueProjects as p}
          <option value={p}>{p}</option>
        {/each}
      </select>

      <!-- User Filter -->
      <select
        bind:value={userFilter}
        class="px-2 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-xs text-slate-300 outline-none cursor-pointer focus:border-indigo-650"
      >
        <option value="all">All Users</option>
        {#each uniqueUsers as u}
          <option value={u}>{u}</option>
        {/each}
      </select>

      <!-- Involvement Filter -->
      <select
        bind:value={involvementFilter}
        class="px-2 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-xs text-slate-300 outline-none cursor-pointer focus:border-indigo-650"
      >
        <option value="all">All Involvement</option>
        <option value="me">Assigned/Authored by Me</option>
      </select>

      <!-- Reset Button -->
      {#if searchQuery !== "" || groupFilter !== "all" || projectFilter !== "all" || userFilter !== "all" || involvementFilter !== "all"}
        <button
          onclick={resetFilters}
          class="px-2.5 py-1.5 text-xs font-semibold text-indigo-400 hover:text-indigo-300 transition"
        >
          Clear Filters
        </button>
      {/if}
    </div>
  {/if}

  <!-- Main View Router -->
  <div class="flex-1 overflow-hidden relative">
    {#if filteredIssues.length === 0}
      <div class="absolute inset-0 flex flex-col items-center justify-center text-center p-6 bg-slate-950/5">
        <svg class="w-12 h-12 text-slate-700/60 mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <h3 class="text-sm font-semibold text-slate-300">No issues found</h3>
        <p class="text-slate-500 text-xs mt-1 max-w-sm">No issues matching your active filter criteria are available.</p>
      </div>
    {:else if viewMode === "list"}
      <!-- List Layout -->
      <div class="h-full overflow-y-auto p-6 space-y-3">
        {#each filteredIssues as issue (issue.id)}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div
            draggable="true"
            ondragstart={(e) => handleDragStart(e, issue)}
            ondragend={handleDragEnd}
            oncontextmenu={(e) => handleContextMenu(e, issue.web_url)}
            class="group p-4 bg-slate-950/20 border border-slate-900/50 rounded-xl flex items-start justify-between hover:bg-slate-900/20 hover:border-slate-800/40 transition duration-150 border-l-4 {getBorderLeftAccent(issue)}"
          >
            <div class="min-w-0 flex-1 pr-4">
              <!-- Project Path -->
              <span class="text-xs font-semibold text-indigo-400/90 tracking-tight truncate block">
                {getProjectPath(issue.web_url)}
              </span>

              <!-- Title & State -->
              <div class="flex items-center space-x-2.5 mt-0.5">
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
                <h3
                  onclick={() => Browser.OpenURL(issue.web_url)}
                  class="text-sm font-semibold text-slate-100 hover:text-indigo-400 cursor-pointer transition line-clamp-1"
                >
                  {issue.title}
                </h3>
                <span class="text-slate-500 text-xs shrink-0 font-mono">#{issue.iid}</span>
              </div>

              <!-- Labels -->
              {#if issue.labels && issue.labels.length > 0}
                <div class="flex flex-wrap gap-1.5 mt-2.5">
                  {#each issue.labels as label}
                    <span
                      class="px-1.5 py-0.5 text-[10px] font-medium rounded border"
                      style="color: {getLabelColorHash(label)}; border-color: {getLabelColorHash(label)}25; bg-color: {getLabelColorHash(label)}08"
                    >
                      {label}
                    </span>
                  {/each}
                </div>
              {/if}

              <!-- Description Preview -->
              {#if issue.description}
                <p class="text-slate-400 text-xs mt-2 line-clamp-2 pr-4 font-normal">
                  {issue.description}
                </p>
              {/if}

              <!-- Metadata -->
              <div class="flex items-center space-x-4 mt-3 text-[10px] text-slate-500 font-medium select-none">
                <span class="flex items-center space-x-1">
                  <span>Created by</span>
                  <span class="text-slate-400 font-semibold">{issue.author?.name}</span>
                </span>
                <span>•</span>
                <span>updated {formatRelativeTime(issue.updated_at)}</span>

                <!-- Comments & Votes -->
                {#if issue.user_notes_count > 0 || issue.upvotes > 0 || issue.downvotes > 0}
                  <span>•</span>
                  <div class="flex items-center space-x-2.5">
                    {#if issue.user_notes_count > 0}
                      <span class="flex items-center space-x-1">
                        <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
                        </svg>
                        <span>{issue.user_notes_count}</span>
                      </span>
                    {/if}
                    {#if issue.upvotes > 0}
                      <span class="flex items-center space-x-1 text-emerald-500/80">
                        <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14 10h4.764a2 2 0 011.789 2.894l-3.5 7A2 2 0 0115.263 21h-4.017c-.163 0-.326-.02-.485-.06L7 20m7-10V5a2 2 0 00-2-2h-.095c-.5 0-.905.405-.905.905 0 .714-.211 1.412-.608 2.006L7 11v9m7-10h-2M7 20H5a2 2 0 01-2-2v-6a2 2 0 012-2h2.5" />
                        </svg>
                        <span>{issue.upvotes}</span>
                      </span>
                    {/if}
                  </div>
                {/if}
              </div>
            </div>

            <!-- Right Controls: Assignees & Inline Close -->
            <div class="flex flex-col items-end justify-between self-stretch shrink-0">
              <!-- Assignees -->
              <div class="flex -space-x-1.5 overflow-hidden">
                {#each issue.assignees || [] as assignee}
                  {#if assignee.avatar_url}
                    <img
                      src={assignee.avatar_url}
                      alt={assignee.name}
                      class="inline-block h-5.5 w-5.5 rounded-full ring-2 ring-slate-950 border border-slate-800 object-cover"
                      title="Assigned to {assignee.name}"
                    />
                  {:else}
                    <div
                      class="inline-block h-5.5 w-5.5 rounded-full ring-2 ring-slate-950 border border-slate-800 bg-slate-800 flex items-center justify-center text-[8px] font-bold text-slate-300"
                      title="Assigned to {assignee.name}"
                    >
                      {assignee.name.substring(0, 2).toUpperCase()}
                    </div>
                  {/if}
                {/each}
              </div>

              <!-- Close Action -->
              {#if issue.state === "opened"}
                <button
                  onclick={() => handleClose(issue.project_id, issue.iid, issue.id)}
                  disabled={processingIssues[issue.id] === "closing"}
                  class="mt-4 px-2 py-1 text-[10px] font-bold rounded bg-rose-600/10 text-rose-400 border border-rose-500/20 hover:bg-rose-600 hover:text-white transition duration-150 disabled:opacity-50 cursor-pointer"
                >
                  {#if processingIssues[issue.id] === "closing"}
                    Closing...
                  {:else}
                    Close Issue
                  {/if}
                </button>
              {/if}
            </div>
          </div>
        {/each}
      </div>
    {:else if viewMode === "kanban"}
      <!-- Kanban Layout -->
      <div class="h-full overflow-x-auto p-6 flex space-x-4 items-start select-none">
        
        <!-- Column 1: Open / Backlog (Unassigned) -->
        <div class="flex-1 min-w-[280px] max-w-[350px] bg-slate-900/15 border border-slate-900/80 rounded-2xl p-3.5 flex flex-col h-full">
          <div class="flex items-center justify-between pb-2 mb-3 border-b border-slate-900/50 shrink-0">
            <h3 class="text-xs font-bold uppercase tracking-wider text-slate-400">Open</h3>
            <span class="px-1.5 py-0.5 text-[10px] font-bold bg-slate-800 text-slate-400 rounded-full">{backlogIssues.length}</span>
          </div>
          <div class="flex-1 overflow-y-auto space-y-2.5 pr-1">
            {#each backlogIssues as issue (issue.id)}
              <!-- svelte-ignore a11y_no_static_element_interactions -->
              <div
                draggable="true"
                ondragstart={(e) => handleDragStart(e, issue)}
                ondragend={handleDragEnd}
                oncontextmenu={(e) => handleContextMenu(e, issue.web_url)}
                class="group p-3 bg-slate-950/20 border border-slate-900/60 hover:border-slate-850/60 rounded-xl cursor-grab active:cursor-grabbing hover:bg-slate-900/15 transition duration-150"
              >
                <div class="text-xs font-semibold text-indigo-400 truncate">{getProjectPath(issue.web_url)}</div>
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
                <h4 onclick={() => Browser.OpenURL(issue.web_url)} class="text-sm font-semibold text-slate-200 hover:text-indigo-400 cursor-pointer mt-1 line-clamp-2">{issue.title}</h4>
                
                {#if issue.labels && issue.labels.length > 0}
                  <div class="flex flex-wrap gap-1 mt-2">
                    {#each issue.labels.slice(0, 3) as label}
                      <span class="px-1.5 py-0.2 text-[8px] font-semibold rounded border" style="color: {getLabelColorHash(label)}; border-color: {getLabelColorHash(label)}20">
                        {label}
                      </span>
                    {/each}
                  </div>
                {/if}

                <div class="flex items-center justify-between mt-3 text-[10px] text-slate-500">
                  <span>#{issue.iid}</span>
                  <span>{formatRelativeTime(issue.updated_at)}</span>
                </div>
              </div>
            {/each}
          </div>
        </div>

        <!-- Column 2: In Progress (Assigned) -->
        <div class="flex-1 min-w-[280px] max-w-[350px] bg-slate-900/15 border border-slate-900/80 rounded-2xl p-3.5 flex flex-col h-full">
          <div class="flex items-center justify-between pb-2 mb-3 border-b border-slate-900/50 shrink-0">
            <h3 class="text-xs font-bold uppercase tracking-wider text-sky-400">In Progress</h3>
            <span class="px-1.5 py-0.5 text-[10px] font-bold bg-sky-500/10 text-sky-400 rounded-full">{inProgressIssues.length}</span>
          </div>
          <div class="flex-1 overflow-y-auto space-y-2.5 pr-1">
            {#each inProgressIssues as issue (issue.id)}
              <!-- svelte-ignore a11y_no_static_element_interactions -->
              <div
                draggable="true"
                ondragstart={(e) => handleDragStart(e, issue)}
                ondragend={handleDragEnd}
                oncontextmenu={(e) => handleContextMenu(e, issue.web_url)}
                class="group p-3 bg-slate-950/20 border border-slate-900/60 hover:border-slate-850/60 rounded-xl cursor-grab active:cursor-grabbing hover:bg-slate-900/15 transition duration-150"
              >
                <div class="text-xs font-semibold text-indigo-400 truncate">{getProjectPath(issue.web_url)}</div>
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
                <h4 onclick={() => Browser.OpenURL(issue.web_url)} class="text-sm font-semibold text-slate-200 hover:text-indigo-400 cursor-pointer mt-1 line-clamp-2">{issue.title}</h4>
                
                {#if issue.labels && issue.labels.length > 0}
                  <div class="flex flex-wrap gap-1 mt-2">
                    {#each issue.labels.slice(0, 3) as label}
                      <span class="px-1.5 py-0.2 text-[8px] font-semibold rounded border" style="color: {getLabelColorHash(label)}; border-color: {getLabelColorHash(label)}20">
                        {label}
                      </span>
                    {/each}
                  </div>
                {/if}

                <div class="flex items-center justify-between mt-3 text-[10px] text-slate-500">
                  <div class="flex items-center space-x-1">
                    <span>#{issue.iid}</span>
                    <span class="text-slate-600">•</span>
                    <span>{formatRelativeTime(issue.updated_at)}</span>
                  </div>
                  <div class="flex -space-x-1">
                    {#each issue.assignees || [] as assignee}
                      <img src={assignee.avatar_url} alt="" class="h-4.5 w-4.5 rounded-full border border-slate-950 object-cover" title={assignee.name} />
                    {/each}
                  </div>
                </div>
              </div>
            {/each}
          </div>
        </div>

        <!-- Column 3: Closed Drop Zone / Recently Closed -->
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div
          ondragover={handleDragOverClose}
          ondragleave={handleDragLeaveClose}
          ondrop={handleDropClose}
          class="flex-1 min-w-[280px] max-w-[350px] rounded-2xl p-3.5 flex flex-col h-full border-2 transition-all duration-200 {isDraggingOverClose ? 'bg-rose-955/10 border-rose-500/80' : draggedIssue ? 'bg-slate-900/10 border-dashed border-rose-500/30' : 'bg-slate-900/15 border-slate-900/80'}"
        >
          <div class="flex items-center justify-between pb-2 mb-3 border-b border-slate-900/50 shrink-0">
            <h3 class="text-xs font-bold uppercase tracking-wider text-rose-450">Closed</h3>
            <span class="px-1.5 py-0.5 text-[10px] font-bold bg-rose-500/10 text-rose-400 rounded-full">{closedIssues.length}</span>
          </div>
          <div class="flex-1 overflow-y-auto space-y-2.5 pr-1">
            {#if draggedIssue && closedIssues.length === 0}
              <div class="h-24 flex flex-col items-center justify-center text-center text-slate-555 border border-dashed border-rose-500/20 rounded-xl bg-rose-500/[0.02] p-4 pointer-events-none">
                <svg class="w-5 h-5 text-rose-550 animate-pulse mb-1.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <div class="text-[10px] font-bold">Drop here to Close Issue</div>
              </div>
            {/if}

            {#each closedIssues as issue (issue.id)}
              <div class="group p-3 bg-slate-950/15 border border-slate-900/40 rounded-xl opacity-60 hover:opacity-95 transition duration-150">
                <div class="text-xs font-semibold text-indigo-400 truncate">{getProjectPath(issue.web_url)}</div>
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
                <h4 onclick={() => Browser.OpenURL(issue.web_url)} class="text-sm font-semibold text-slate-200 hover:text-indigo-400 cursor-pointer mt-1 line-clamp-2">{issue.title}</h4>
                <div class="flex items-center justify-between mt-3 text-[10px] text-slate-500">
                  <span>#{issue.iid}</span>
                  <span>closed {formatRelativeTime(issue.closed_at || issue.updated_at)}</span>
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
