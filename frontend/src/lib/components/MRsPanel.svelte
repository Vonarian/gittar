<script lang="ts">
  import type { MergeRequest } from "../../../bindings/gittar/internal/gitlab/models";
  import { Browser } from "@wailsio/runtime";

  interface Props {
    mergeRequests: MergeRequest[];
    username: string;
  }

  let { mergeRequests = [], username }: Props = $props();

  let activeMRTab = $state<"all" | "assigned" | "authored" | "review">("all");

  // Search & Filter state variables
  let searchQuery = $state("");
  let groupFilter = $state("all");
  let projectFilter = $state("all");
  let userFilter = $state("all");

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

  function resetFilters() {
    searchQuery = "";
    groupFilter = "all";
    projectFilter = "all";
    userFilter = "all";
  }

  function formatDate(dateStr: any): string {
    if (!dateStr) return "";
    return new Date(dateStr).toLocaleDateString(undefined, {
      month: "short",
      day: "numeric",
    });
  }
</script>

<div class="h-full flex flex-col">
  <!-- Panel Header -->
  <div class="p-6 border-b border-slate-900/60 flex items-center justify-between">
    <div>
      <h2 class="text-xl font-semibold text-white">MR Gatekeeper</h2>
      <p class="text-slate-400 text-xs mt-1">Aggregate merge requests assigned to, reviewed by, or authored by you.</p>
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
        <span class="px-1.5 py-0.5 text-[10px] bg-slate-800 text-slate-350 rounded-full">
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
        <span class="px-1.5 py-0.5 text-[10px] bg-slate-800 text-slate-350 rounded-full">
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
        {#each uniqueGroups as grp}
          <option value={grp}>{grp}</option>
        {/each}
      </select>

      <!-- Project Filter -->
      <select
        bind:value={projectFilter}
        class="px-2 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-xs text-slate-300 outline-none cursor-pointer focus:border-indigo-650"
      >
        <option value="all">All Projects</option>
        {#each uniqueProjects as proj}
          <option value={proj}>{proj}</option>
        {/each}
      </select>

      <!-- User Filter -->
      <select
        bind:value={userFilter}
        class="px-2 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-xs text-slate-300 outline-none cursor-pointer focus:border-indigo-650"
      >
        <option value="all">All Users</option>
        {#each uniqueUsers as usr}
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
  <div class="flex-1 overflow-y-auto p-6">
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
    {:else if filteredMRs.length === 0}
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
    {:else}
      <div class="space-y-3">
        {#each filteredMRs as mr (mr.id)}
          <div class="p-4 bg-slate-900/30 border border-slate-900/70 hover:border-slate-850/80 rounded-xl transition duration-150 flex items-start justify-between">
            <div class="min-w-0 pr-6">
              <!-- Title & Draft Indicator -->
              <div class="flex items-center space-x-2.5 flex-wrap gap-y-1">
                {#if mr.work_in_progress || mr.draft}
                  <span class="px-1.5 py-0.5 text-[10px] font-bold bg-slate-800 text-slate-400 border border-slate-700/60 rounded">
                    DRAFT
                  </span>
                {/if}
                <span class="text-sm font-semibold text-slate-100 truncate hover:text-indigo-400 transition" title={mr.title}>
                  {mr.title}
                </span>
                <span class="text-xs font-mono text-slate-500">
                  #{mr.iid}
                </span>
              </div>

              <!-- Branches Info -->
              <div class="flex items-center space-x-2 text-xs text-slate-400 mt-2 font-mono">
                <span class="bg-slate-950/70 border border-slate-900 px-1.5 py-0.5 rounded text-[11px] text-slate-350">
                  {mr.source_branch}
                </span>
                <span class="text-slate-650">→</span>
                <span class="bg-slate-950/70 border border-slate-900 px-1.5 py-0.5 rounded text-[11px] text-slate-350">
                  {mr.target_branch}
                </span>
              </div>

              <!-- Metadata & Metrics -->
              <div class="flex items-center space-x-4 mt-3 text-xs text-slate-500">
                <div class="flex items-center space-x-1">
                  {#if mr.author?.avatar_url}
                    <img src={mr.author.avatar_url} alt={mr.author.name} class="w-4.5 h-4.5 rounded-full border border-slate-800" />
                  {/if}
                  <span class="text-slate-350">{mr.author?.name}</span>
                </div>
                
                <span class="text-slate-700">•</span>
                <span>Created {formatDate(mr.created_at)}</span>
                
                <!-- Comments count -->
                {#if mr.user_notes_count > 0}
                  <span class="text-slate-700">•</span>
                  <span class="flex items-center space-x-1 text-slate-400">
                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
                    </svg>
                    <span>{mr.user_notes_count}</span>
                  </span>
                {/if}

                <!-- Upvote count -->
                {#if mr.upvotes > 0}
                  <span class="text-slate-700">•</span>
                  <span class="flex items-center space-x-1 text-emerald-400">
                    <span>👍</span>
                    <span>{mr.upvotes}</span>
                  </span>
                {/if}
              </div>
            </div>

            <!-- Open in Browser Button -->
            <button
              onclick={() => Browser.OpenURL(mr.web_url)}
              class="px-3 py-1.5 border border-slate-800 hover:border-slate-700 bg-slate-950/40 rounded-lg text-slate-400 hover:text-slate-200 transition shrink-0 self-center"
              title="Open Merge Request in Browser"
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
              </svg>
            </button>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>
