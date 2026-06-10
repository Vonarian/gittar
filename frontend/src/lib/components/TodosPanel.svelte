<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import type { Todo } from "../../../bindings/gittar/internal/gitlab/models";
  import { Browser, Clipboard } from "@wailsio/runtime";

  interface Props {
    todos: Todo[];
  }

  let { todos = [] }: Props = $props();

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

  onMount(() => {
    window.addEventListener("click", closeContextMenu);
    window.addEventListener("contextmenu", closeContextMenu);
  });

  onDestroy(() => {
    window.removeEventListener("click", closeContextMenu);
    window.removeEventListener("contextmenu", closeContextMenu);
  });

  // Search & Filter state variables
  let searchQuery = $state("");
  let groupFilter = $state("all");
  let projectFilter = $state("all");
  let actionFilter = $state("all");
  let userFilter = $state("all");

  // Dynamically extract unique values from the todo list to populate filters
  const uniqueGroups = $derived([
    ...new Set(todos.map((t) => t.project?.path_with_namespace?.split("/")[0]).filter(Boolean)),
  ]);

  const uniqueProjects = $derived([
    ...new Set(todos.map((t) => t.project?.path_with_namespace).filter(Boolean)),
  ]);

  const uniqueActions = $derived([
    ...new Set(todos.map((t) => t.action_name).filter(Boolean)),
  ]);

  const uniqueUsers = $derived([
    ...new Set(todos.map((t) => t.author?.name).filter(Boolean)),
  ]);

  // Derived filtered todos list
  const filteredTodos = $derived(
    todos.filter((todo) => {
      const matchesSearch =
        searchQuery === "" ||
        (todo.body || "").toLowerCase().includes(searchQuery.toLowerCase()) ||
        (todo.project?.path_with_namespace || "").toLowerCase().includes(searchQuery.toLowerCase());

      const matchesGroup =
        groupFilter === "all" || todo.project?.path_with_namespace?.split("/")[0] === groupFilter;

      const matchesProject =
        projectFilter === "all" || todo.project?.path_with_namespace === projectFilter;

      const matchesAction =
        actionFilter === "all" || todo.action_name === actionFilter;

      const matchesUser =
        userFilter === "all" || todo.author?.name === userFilter;

      return matchesSearch && matchesGroup && matchesProject && matchesAction && matchesUser;
    })
  );

  function resetFilters() {
    searchQuery = "";
    groupFilter = "all";
    projectFilter = "all";
    actionFilter = "all";
    userFilter = "all";
  }

  // Helper to format date
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

  // Action badge mapping
  function getActionBadgeStyle(action: string): string {
    const act = action.toLowerCase();
    if (act.includes("assigned")) return "bg-blue-500/10 text-blue-400 border-blue-500/20";
    if (act.includes("mention")) return "bg-purple-500/10 text-purple-400 border-purple-500/20";
    if (act.includes("build") || act.includes("fail")) return "bg-rose-500/10 text-rose-400 border-rose-500/20";
    if (act.includes("approval") || act.includes("review")) return "bg-amber-500/10 text-amber-400 border-amber-500/20";
    return "bg-slate-500/10 text-slate-400 border-slate-500/20";
  }

  function formatActionLabel(action: string): string {
    return action.replace(/_/g, " ");
  }
</script>

<div class="h-full flex flex-col">
  <!-- Panel Header -->
  <div class="p-6 border-b border-slate-900/60 flex items-center justify-between">
    <div>
      <h2 class="text-xl font-semibold text-white flex items-center">
        Inbox Feed
        {#if todos.length > 0}
          <span class="ml-2.5 px-2 py-0.5 text-xs font-semibold bg-indigo-500/10 text-indigo-400 rounded-full border border-indigo-500/20">
            {todos.length}
          </span>
        {/if}
      </h2>
      <p class="text-slate-400 text-xs mt-1">Pending actions and notifications requiring your immediate attention.</p>
    </div>
  </div>

  <!-- Filter & Search Toolbar -->
  {#if todos.length > 0}
    <div class="px-6 py-3 border-b border-slate-900/40 bg-slate-950/20 flex flex-wrap items-center gap-3 select-none">
      <!-- Search Input -->
      <div class="relative w-48">
        <input
          type="text"
          bind:value={searchQuery}
          placeholder="Search inbox..."
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

      <!-- Action Filter -->
      <select
        bind:value={actionFilter}
        class="px-2 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-xs text-slate-300 outline-none cursor-pointer focus:border-indigo-650"
      >
        <option value="all">All Actions</option>
        {#each uniqueActions as action (action)}
          <option value={action}>{formatActionLabel(action)}</option>
        {/each}
      </select>

      <!-- Author Filter -->
      <select
        bind:value={userFilter}
        class="px-2 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-xs text-slate-300 outline-none cursor-pointer focus:border-indigo-650"
      >
        <option value="all">All Authors</option>
        {#each uniqueUsers as user (user)}
          <option value={user}>{user}</option>
        {/each}
      </select>

      <!-- Reset Filters Button -->
      {#if searchQuery !== "" || groupFilter !== "all" || projectFilter !== "all" || actionFilter !== "all" || userFilter !== "all"}
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
    {#if todos.length === 0}
      <!-- Premium Inbox Zero State -->
      <div class="h-[70%] flex flex-col items-center justify-center text-center">
        <div class="w-12 h-12 rounded-full bg-indigo-950/40 border border-indigo-900/50 flex items-center justify-center text-indigo-400 mb-4 animate-pulse-glow">
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </div>
        <h3 class="text-base font-semibold text-slate-200">Inbox Zero</h3>
        <p class="text-slate-500 text-sm mt-1 max-w-[280px]">You are completely caught up! No pending todos on your queue.</p>
      </div>
    {:else if filteredTodos.length === 0}
      <!-- Empty Filter State -->
      <div class="h-[70%] flex flex-col items-center justify-center text-center">
        <div class="w-12 h-12 rounded-full bg-slate-950/40 border border-slate-900 flex items-center justify-center text-slate-500 mb-4">
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
          </svg>
        </div>
        <h3 class="text-base font-semibold text-slate-350">No Results Found</h3>
        <p class="text-slate-500 text-sm mt-1 max-w-[280px]">No todos match your filter criteria. Try resetting or adjusting your search.</p>
        <button
          onclick={resetFilters}
          class="mt-4 px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-lg transition"
        >
          Reset Filters
        </button>
      </div>
    {:else}
      <!-- High Density List -->
      <div class="space-y-2.5">
        {#each filteredTodos as todo (todo.id)}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div
            oncontextmenu={(e) => handleContextMenu(e, todo.target_url)}
            class="group flex items-start justify-between p-3.5 bg-slate-900/30 border border-slate-900/70 hover:border-slate-800/80 rounded-xl transition duration-150 relative"
          >
            <div class="flex items-start space-x-3.5 min-w-0 pr-4">
              <!-- Author Avatar -->
              {#if todo.author?.avatar_url}
                <img src={todo.author.avatar_url} alt={todo.author.name} class="w-8 h-8 rounded-full border border-slate-800 mt-0.5" />
              {:else}
                <div class="w-8 h-8 rounded-full bg-slate-800 flex items-center justify-center text-xs font-semibold text-slate-400 border border-slate-700 mt-0.5">
                  {todo.author?.name?.charAt(0) || "U"}
                </div>
              {/if}

              <div class="min-w-0">
                <!-- Project & Author Info -->
                <div class="flex items-center space-x-2 flex-wrap gap-y-1">
                  <span class="text-xs font-semibold text-indigo-400 truncate max-w-[180px]">
                    {todo.project?.path_with_namespace}
                  </span>
                  <span class="text-slate-650 text-[10px]">•</span>
                  <span class="text-xs text-slate-300">
                    {todo.author?.name}
                  </span>
                  <span class="text-slate-650 text-[10px]">•</span>
                  <span class="text-[10px] text-slate-500">
                    {formatRelativeTime(todo.created_at)}
                  </span>
                </div>

                <!-- Todo Body -->
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <!-- svelte-ignore a11y_no_static_element_interactions -->
                <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
                <p
                  onclick={() => Browser.OpenURL(todo.target_url)}
                  class="text-sm text-slate-200 mt-1.5 font-medium leading-relaxed truncate max-w-[580px] hover:text-indigo-400 transition cursor-pointer"
                  title={todo.body}
                >
                  {todo.body}
                </p>

                <!-- Action Badge & Target Type -->
                <div class="flex items-center space-x-2 mt-2">
                  <span class="px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider rounded border {getActionBadgeStyle(todo.action_name)}">
                    {formatActionLabel(todo.action_name)}
                  </span>
                  <span class="text-[10px] font-semibold text-slate-500 font-mono">
                    {todo.target_type}
                  </span>
                </div>
              </div>
            </div>

            <!-- Hover Action Button -->
            <div class="opacity-0 group-hover:opacity-100 transition duration-150 self-center flex items-center pr-1.5">
              <button
                onclick={(e) => { e.stopPropagation(); Browser.OpenURL(todo.target_url); }}
                class="flex items-center space-x-1.5 px-3 py-1.5 bg-slate-850 hover:bg-slate-850 border border-slate-700 hover:border-slate-600 text-xs font-medium text-slate-200 rounded-lg transition"
              >
                <span>View</span>
                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                </svg>
              </button>
            </div>
          </div>
        {/each}
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
