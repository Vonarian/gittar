<script lang="ts">
  import { Window } from "@wailsio/runtime";

  interface Props {
    currentTab: string;
    todosCount: number;
    runningPipelines: number;
    failedPipelines: number;
    mrsCount: number;
    username: string;
    avatarUrl: string;
    syncError: string;
    isSyncing?: boolean;
    onSelectTab: (tab: string) => void;
  }

  let {
    currentTab,
    todosCount,
    runningPipelines,
    failedPipelines,
    mrsCount,
    username,
    avatarUrl,
    syncError,
    isSyncing = false,
    onSelectTab,
  }: Props = $props();

  function handleDoubleClickTitlebar() {
    Window.ToggleMaximise();
  }
</script>

<div class="h-screen w-[240px] bg-slate-950/85 border-r border-slate-900/50 flex flex-col justify-between select-none backdrop-blur-md">
  <!-- Profile & Header -->
  <div>
    <!-- Drag area spacer for macOS traffic lights -->
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="h-10 shrink-0 select-none cursor-default"
      style="-webkit-app-region: drag; --wails-draggable: drag;"
      role="none"
      ondblclick={handleDoubleClickTitlebar}
    ></div>

    <div class="p-4 flex items-center space-x-3 border-b border-slate-900/60">
      {#if avatarUrl}
        <img src={avatarUrl} alt="Avatar" class="w-8 h-8 rounded-full border border-slate-800" />
      {:else}
        <img src="/gittar_logo.png" alt="Gittar" class="w-8 h-8 rounded-full border border-slate-800 object-cover" />
      {/if}
      <div class="min-w-0 flex-1">
        <h1 class="text-sm font-semibold text-slate-100 truncate">Gittar</h1>
        {#if isSyncing}
          <p class="text-[10px] text-indigo-400 font-semibold truncate flex items-center">
            <span class="w-1.5 h-1.5 rounded-full bg-indigo-500 mr-1.5 shrink-0 animate-ping"></span>
            Syncing...
          </p>
        {:else if syncError}
          <p class="text-[10px] text-amber-500 font-semibold truncate flex items-center" title={syncError}>
            <span class="w-1.5 h-1.5 rounded-full bg-amber-500 mr-1.5 shrink-0 animate-pulse"></span>
            Offline (cached)
          </p>
        {:else}
          <p class="text-xs text-slate-400 truncate">@{username || "unconnected"}</p>
        {/if}
      </div>
      <div class="flex items-center">
        <span class="relative flex h-2 w-2">
          {#if username}
            <span class="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
            <span class="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
          {:else}
            <span class="relative inline-flex rounded-full h-2 w-2 bg-slate-600"></span>
          {/if}
        </span>
      </div>
    </div>

    <!-- Navigation Menu -->
    <nav class="p-3 space-y-1">
      <!-- Todos -->
      <button
        onclick={() => onSelectTab("todos")}
        class="w-full flex items-center justify-between px-3 py-2 text-sm rounded-lg transition-all border {currentTab === 'todos' ? 'bg-indigo-600/15 text-indigo-200 border-indigo-500/20 font-medium shadow-sm shadow-indigo-500/5' : 'text-slate-400 hover:bg-slate-900/40 hover:text-slate-200 border-transparent'}"
      >
        <div class="flex items-center space-x-2.5">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
          </svg>
          <span>Inbox Feed</span>
        </div>
        {#if todosCount > 0}
          <span class="px-2 py-0.5 text-xs font-semibold bg-indigo-500/15 text-indigo-400 rounded-full border border-indigo-500/25">
            {todosCount}
          </span>
        {/if}
      </button>

      <!-- Pipelines -->
      <button
        onclick={() => onSelectTab("pipelines")}
        class="w-full flex items-center justify-between px-3 py-2 text-sm rounded-lg transition-all border {currentTab === 'pipelines' ? 'bg-indigo-600/15 text-indigo-200 border-indigo-500/20 font-medium shadow-sm shadow-indigo-500/5' : 'text-slate-400 hover:bg-slate-900/40 hover:text-slate-200 border-transparent'}"
      >
        <div class="flex items-center space-x-2.5">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z" />
          </svg>
          <span>Pipelines Matrix</span>
        </div>
        <div class="flex items-center space-x-1">
          {#if failedPipelines > 0}
            <span class="px-1.5 py-0.5 text-[10px] font-bold bg-rose-500/15 text-rose-400 rounded border border-rose-500/20">
              {failedPipelines}
            </span>
          {/if}
          {#if runningPipelines > 0}
            <span class="w-2 h-2 rounded-full bg-amber-500 animate-pulse-glow"></span>
          {/if}
        </div>
      </button>

      <!-- MRs -->
      <button
        onclick={() => onSelectTab("mrs")}
        class="w-full flex items-center justify-between px-3 py-2 text-sm rounded-lg transition-all border {currentTab === 'mrs' ? 'bg-indigo-600/15 text-indigo-200 border-indigo-500/20 font-medium shadow-sm shadow-indigo-500/5' : 'text-slate-400 hover:bg-slate-900/40 hover:text-slate-200 border-transparent'}"
      >
        <div class="flex items-center space-x-2.5">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
          </svg>
          <span>MR Gatekeeper</span>
        </div>
        {#if mrsCount > 0}
          <span class="px-2 py-0.5 text-xs font-semibold bg-emerald-500/10 text-emerald-400 rounded-full border border-emerald-500/20">
            {mrsCount}
          </span>
        {/if}
      </button>
    </nav>
  </div>

  <!-- Connection & Settings Footer -->
  <div class="p-3 border-t border-slate-900/60 bg-slate-950/40">
    <button
      onclick={() => onSelectTab("setup")}
      class="w-full flex items-center space-x-2.5 px-3 py-2 text-sm rounded-lg transition-all border {currentTab === 'setup' ? 'bg-indigo-600/15 text-indigo-200 border-indigo-500/20 font-medium shadow-sm shadow-indigo-500/5' : 'text-slate-400 hover:bg-slate-900/40 hover:text-slate-200 border-transparent'}"
    >
      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
      </svg>
      <span>Settings</span>
    </button>

    <div class="mt-2 text-[10px] text-slate-500 text-center uppercase tracking-wider font-semibold">
      Gittar Control Panel
    </div>
  </div>
</div>
