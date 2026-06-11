<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import type { Todo } from "../../../bindings/gittar/internal/gitlab/models";
  import { Browser, Clipboard } from "@wailsio/runtime";
  import { MarkTodoAsDone } from "../../../bindings/gittar/internal/service/appservice";

  interface Props {
    todos: Todo[];
    onRefresh?: () => void;
  }

  let { todos = [], onRefresh }: Props = $props();

  // View mode: 'list' or 'kanban'
  let viewMode = $state("list");

  // Drag and drop state
  let draggedTodo = $state<Todo | null>(null);
  let isDraggingOverDone = $state(false);

  // Local session cache for completed todos
  let recentlyDoneTodos = $state<Todo[]>([]);
  let isMarkingDone = $state<Record<number, boolean>>({});

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
  let actionFilter = $state("all");
  let userFilter = $state("all");

  // Load view mode and filters from localStorage on mount
  onMount(() => {
    viewMode = localStorage.getItem("gittar_view_mode_todos") || "list";
    searchQuery = localStorage.getItem("gittar_filter_todos_search") || "";
    groupFilter = localStorage.getItem("gittar_filter_todos_group") || "all";
    projectFilter = localStorage.getItem("gittar_filter_todos_project") || "all";
    actionFilter = localStorage.getItem("gittar_filter_todos_action") || "all";
    userFilter = localStorage.getItem("gittar_filter_todos_user") || "all";

    window.addEventListener("click", closeContextMenu);
    window.addEventListener("contextmenu", closeContextMenu);
  });

  onDestroy(() => {
    window.removeEventListener("click", closeContextMenu);
    window.removeEventListener("contextmenu", closeContextMenu);
  });

  // Persist view mode and filters using effects whenever they change
  $effect(() => {
    localStorage.setItem("gittar_view_mode_todos", viewMode);
  });

  $effect(() => {
    localStorage.setItem("gittar_filter_todos_search", searchQuery);
    localStorage.setItem("gittar_filter_todos_group", groupFilter);
    localStorage.setItem("gittar_filter_todos_project", projectFilter);
    localStorage.setItem("gittar_filter_todos_action", actionFilter);
    localStorage.setItem("gittar_filter_todos_user", userFilter);
  });

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

  // API Call to mark a todo as done/read
  async function markAsDone(todoId: number) {
    if (isMarkingDone[todoId]) return;
    isMarkingDone[todoId] = true;
    try {
      const found = todos.find((t) => t.id === todoId);
      await MarkTodoAsDone(todoId);
      if (found) {
        recentlyDoneTodos = [found, ...recentlyDoneTodos];
      }
      if (onRefresh) {
        onRefresh();
      }
    } catch (e: any) {
      console.error("Failed to mark todo as done:", e);
      alert("Failed to mark todo as done: " + e.message);
    } finally {
      isMarkingDone[todoId] = false;
    }
  }

  // Drag and Drop Event Handlers
  function handleDragStart(e: DragEvent, todo: Todo) {
    draggedTodo = todo;
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = "move";
      e.dataTransfer.setData("text/plain", todo.id.toString());
    }
  }

  function handleDragEnd() {
    draggedTodo = null;
    isDraggingOverDone = false;
  }

  function handleDragOverDone(e: DragEvent) {
    e.preventDefault();
    isDraggingOverDone = true;
  }

  function handleDragLeaveDone() {
    isDraggingOverDone = false;
  }

  async function handleDropDone(e: DragEvent) {
    e.preventDefault();
    isDraggingOverDone = false;
    if (draggedTodo) {
      const todoId = draggedTodo.id;
      await markAsDone(todoId);
      draggedTodo = null;
    }
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

  function getBorderLeftAccent(action: string): string {
    const act = action.toLowerCase();
    if (act.includes("assigned")) return "border-l-blue-500/40";
    if (act.includes("mention")) return "border-l-purple-500/40";
    if (act.includes("build") || act.includes("fail")) return "border-l-rose-500/40";
    if (act.includes("approval") || act.includes("review")) return "border-l-amber-500/40";
    return "border-l-slate-800/60";
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
  <div class="flex-1 overflow-hidden relative">
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
    {:else if filteredTodos.length === 0 && viewMode === "list"}
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
    {:else if viewMode === "list"}
      <!-- High Density List -->
      <div class="h-full overflow-y-auto p-6 space-y-2.5">
        {#each filteredTodos as todo (todo.id)}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div
            oncontextmenu={(e) => handleContextMenu(e, todo.target_url)}
            class="group flex items-start justify-between p-3.5 bg-slate-950/20 border-t border-r border-b border-l-2 border-y-slate-900/40 border-r-slate-900/40 {getBorderLeftAccent(todo.action_name)} hover:bg-slate-900/25 hover:border-y-slate-800/60 hover:border-r-slate-800/60 hover:shadow-md hover:shadow-indigo-500/5 rounded-xl transition-all duration-200 relative"
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

            <!-- Hover Action Button & Mark as Done -->
            <div class="opacity-0 group-hover:opacity-100 transition duration-150 self-center flex items-center space-x-2 pr-1.5">
              <button
                onclick={(e) => { e.stopPropagation(); markAsDone(todo.id); }}
                disabled={isMarkingDone[todo.id]}
                class="flex items-center justify-center p-2 bg-emerald-600/10 hover:bg-emerald-600 border border-emerald-500/30 hover:border-emerald-500 text-emerald-400 hover:text-white rounded-lg transition disabled:opacity-40"
                title="Mark as Done"
              >
                {#if isMarkingDone[todo.id]}
                  <div class="w-3.5 h-3.5 border-2 border-emerald-400 border-t-transparent rounded-full animate-spin"></div>
                {:else}
                  <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M5 13l4 4L19 7" />
                  </svg>
                {/if}
              </button>

              <button
                onclick={(e) => { e.stopPropagation(); Browser.OpenURL(todo.target_url); }}
                class="flex items-center space-x-1.5 px-3 py-1.5 bg-slate-850 hover:bg-slate-800 border border-slate-700 hover:border-slate-600 text-xs font-medium text-slate-200 rounded-lg transition"
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
    {:else}
      <!-- Kanban Board View -->
      <div class="h-full w-full overflow-x-auto p-6 flex space-x-6">
        <!-- Column 1: Pending Inbox -->
        <div class="w-1/2 flex flex-col bg-slate-900/15 border border-slate-900/80 rounded-2xl p-4 min-w-[320px] max-w-[500px]">
          <div class="flex items-center justify-between mb-4 pb-2 border-b border-slate-900/60 shrink-0">
            <div class="flex items-center space-x-2">
              <span class="w-2 h-2 rounded-full bg-indigo-500"></span>
              <h3 class="font-bold text-slate-200 text-sm">Pending Inbox</h3>
            </div>
            <span class="px-2 py-0.5 text-xs font-semibold bg-indigo-500/10 text-indigo-400 rounded-full border border-indigo-500/20">
              {filteredTodos.length}
            </span>
          </div>

          <div class="flex-1 overflow-y-auto space-y-3 pr-1">
            {#if filteredTodos.length === 0}
              <div class="h-32 flex flex-col items-center justify-center text-center text-slate-500 text-xs border border-dashed border-slate-800/80 rounded-xl bg-slate-950/5">
                No pending items.
              </div>
            {/if}

            {#each filteredTodos as todo (todo.id)}
              <!-- svelte-ignore a11y_no_static_element_interactions -->
              <div
                draggable="true"
                ondragstart={(e) => handleDragStart(e, todo)}
                ondragend={handleDragEnd}
                class="group flex flex-col p-3.5 bg-slate-950/35 hover:bg-slate-950/60 border border-slate-900/60 hover:border-slate-800/70 hover:shadow-md hover:shadow-indigo-500/5 rounded-xl cursor-grab active:cursor-grabbing transition-all duration-200 relative select-none"
              >
                <!-- Drag Grip Icon -->
                <div class="absolute right-3 top-3 opacity-0 group-hover:opacity-40 transition duration-150 text-slate-400">
                  <svg class="w-3.5 h-3.5" viewBox="0 0 20 20" fill="currentColor">
                    <path d="M7 2a2 2 0 11-4 0 2 2 0 014 0zM7 8a2 2 0 11-4 0 2 2 0 014 0zM7 14a2 2 0 11-4 0 2 2 0 014 0zM13 2a2 2 0 11-4 0 2 2 0 014 0zM13 8a2 2 0 11-4 0 2 2 0 014 0zM13 14a2 2 0 11-4 0 2 2 0 014 0z" />
                  </svg>
                </div>

                <div class="flex items-center space-x-2">
                  {#if todo.author?.avatar_url}
                    <img src={todo.author.avatar_url} alt={todo.author.name} class="w-5 h-5 rounded-full border border-slate-800" />
                  {:else}
                    <div class="w-5 h-5 rounded-full bg-slate-800 flex items-center justify-center text-[10px] font-semibold text-slate-400 border border-slate-700">
                      {todo.author?.name?.charAt(0) || "U"}
                    </div>
                  {/if}
                  <span class="text-xs text-slate-450 truncate max-w-[140px] font-medium">{todo.author?.name}</span>
                  <span class="text-slate-650 text-[10px]">•</span>
                  <span class="text-[10px] text-slate-500">{formatRelativeTime(todo.created_at)}</span>
                </div>

                <div class="text-xs text-indigo-400 font-semibold mt-1.5 truncate max-w-[240px]">
                  {todo.project?.path_with_namespace}
                </div>

                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <p
                  onclick={() => Browser.OpenURL(todo.target_url)}
                  class="text-sm text-slate-200 mt-2 font-medium leading-normal hover:text-indigo-400 transition cursor-pointer select-none line-clamp-2"
                  title={todo.body}
                >
                  {todo.body}
                </p>

                <div class="flex items-center justify-between mt-3">
                  <span class="px-2 py-0.5 text-[9px] font-bold uppercase tracking-wider rounded border {getActionBadgeStyle(todo.action_name)}">
                    {formatActionLabel(todo.action_name)}
                  </span>

                  <button
                    onclick={(e) => { e.stopPropagation(); markAsDone(todo.id); }}
                    disabled={isMarkingDone[todo.id]}
                    class="flex items-center space-x-1 px-2.5 py-1 bg-emerald-650/10 hover:bg-emerald-650 border border-emerald-500/25 hover:border-emerald-500 text-emerald-400 hover:text-white rounded-md text-[10px] font-bold transition disabled:opacity-40"
                  >
                    {#if isMarkingDone[todo.id]}
                      <div class="w-3 h-3 border border-emerald-400 border-t-transparent rounded-full animate-spin"></div>
                    {:else}
                      <span>Done</span>
                    {/if}
                  </button>
                </div>
              </div>
            {/each}
          </div>
        </div>

        <!-- Column 2: Mark as Done / Recently Completed -->
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div
          ondragover={handleDragOverDone}
          ondragleave={handleDragLeaveDone}
          ondrop={handleDropDone}
          class="w-1/2 flex flex-col rounded-2xl p-4 min-w-[320px] max-w-[500px] transition-all duration-200 border-2 select-none {isDraggingOverDone ? 'bg-indigo-650/5 border-indigo-500/80 shadow-lg shadow-indigo-600/5' : draggedTodo ? 'bg-slate-900/10 border-dashed border-indigo-500/30' : 'bg-slate-900/15 border-slate-900/80'}"
        >
          <div class="flex items-center justify-between mb-4 pb-2 border-b border-slate-900/60 shrink-0">
            <div class="flex items-center space-x-2">
              <span class="w-2 h-2 rounded-full bg-emerald-500"></span>
              <h3 class="font-bold text-slate-200 text-sm">Mark as Done / Read</h3>
            </div>
            {#if recentlyDoneTodos.length > 0}
              <span class="px-2 py-0.5 text-xs font-semibold bg-emerald-500/10 text-emerald-400 rounded-full border border-emerald-500/20">
                {recentlyDoneTodos.length}
              </span>
            {/if}
          </div>

          <!-- Drag and Drop Drop Zone Overlay -->
          {#if draggedTodo && recentlyDoneTodos.length === 0}
            <div class="flex-1 flex flex-col items-center justify-center text-center p-6 pointer-events-none">
              <div class="w-14 h-14 rounded-full border-2 border-dashed border-indigo-500/50 flex items-center justify-center text-indigo-400 mb-3 animate-pulse">
                <svg class="w-7 h-7" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </div>
              <h4 class="text-sm font-semibold text-slate-250">Drop card to Mark as Done</h4>
              <p class="text-slate-500 text-xs mt-1 max-w-[180px]">Drop files here to mark them read on GitLab.</p>
            </div>
          {:else}
            <div class="flex-1 overflow-y-auto space-y-3 pr-1">
              <!-- Drop instruction visual helper when drag active and items exist -->
              {#if draggedTodo}
                <div class="flex items-center justify-center p-3 border-2 border-dashed border-indigo-500/40 rounded-xl bg-indigo-500/5 text-xs font-bold text-indigo-300 text-center animate-pulse pointer-events-none">
                  Drop card here to Complete
                </div>
              {/if}

              {#if recentlyDoneTodos.length === 0}
                <div class="h-32 flex flex-col items-center justify-center text-center text-slate-500 text-xs border border-dashed border-slate-850 rounded-xl bg-slate-950/5">
                  Drag items from "Pending Inbox" and drop them here to resolve.
                </div>
              {:else}
                {#each recentlyDoneTodos as todo (todo.id)}
                  <div class="flex flex-col p-3.5 bg-slate-950/15 border border-slate-900/40 rounded-xl relative opacity-60 hover:opacity-95 transition-opacity select-none duration-150">
                    <div class="flex items-center justify-between">
                      <div class="flex items-center space-x-2">
                        {#if todo.author?.avatar_url}
                          <img src={todo.author.avatar_url} alt={todo.author.name} class="w-5 h-5 rounded-full border border-slate-800" />
                        {/if}
                        <span class="text-xs text-slate-450 truncate max-w-[120px] font-medium">{todo.author?.name}</span>
                      </div>
                      <span class="px-2 py-0.5 text-[9px] font-extrabold bg-emerald-500/10 text-emerald-400 border border-emerald-500/25 rounded">
                        COMPLETED
                      </span>
                    </div>

                    <div class="text-[11px] text-indigo-500 font-semibold mt-1.5 truncate max-w-[240px]">
                      {todo.project?.path_with_namespace}
                    </div>

                    <!-- svelte-ignore a11y_click_events_have_key_events -->
                    <p
                      onclick={() => Browser.OpenURL(todo.target_url)}
                      class="text-xs text-slate-350 mt-2 font-medium leading-normal hover:text-indigo-400 transition cursor-pointer select-none line-clamp-2"
                      title={todo.body}
                    >
                      {todo.body}
                    </p>
                  </div>
                {/each}
              {/if}
            </div>
          {/if}
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
