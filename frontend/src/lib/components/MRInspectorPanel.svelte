<script lang="ts">
  import { untrack, onMount } from "svelte";
  import type { MergeRequest, Commit, Note } from "../../../bindings/gittar/internal/gitlab/models";
  import { 
    GetMergeRequestCommits, 
    GetMergeRequestNotes, 
    CreateMergeRequestNote,
    GetSingleMergeRequest,
    MergeMergeRequest,
    CloseMergeRequest
  } from "../../../bindings/gittar/internal/service/appservice";
  import { Browser } from "@wailsio/runtime";

  interface Props {
    isOpen: boolean;
    mr: MergeRequest | null;
    username: string;
    onClose: () => void;
    onRefreshList?: () => void;
  }

  let { isOpen, mr, username, onClose, onRefreshList }: Props = $props();

  // Selected tab in the drawer: 'overview' | 'commits' | 'comments'
  let activeTab = $state<"overview" | "commits" | "comments">("overview");

  // State
  let detailedMR = $state<MergeRequest | null>(null);
  let commits = $state<Commit[]>([]);
  let notes = $state<Note[]>([]);
  
  let isLoadingMR = $state(false);
  let isLoadingCommits = $state(false);
  let isLoadingNotes = $state(false);
  let isPostingComment = $state(false);
  let isProcessingAction = $state<"merging" | "closing" | null>(null);
  let isActivityExpanded = $state(true);

  onMount(() => {
    (window as any).__openExternal = (url: string) => {
      Browser.OpenURL(url);
    };
  });
  
  let newCommentText = $state("");
  let mrError = $state("");
  let commitsError = $state("");
  let notesError = $state("");

  // Helpers
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

  function getLabelColorHash(label: string): string {
    let hash = 0;
    for (let i = 0; i < label.length; i++) {
      hash = label.charCodeAt(i) + ((hash << 5) - hash);
    }
    const h = Math.abs(hash % 360);
    return `hsl(${h}, 70%, 65%)`;
  }

  // Basic Markdown-to-HTML parser for descriptions and comments
  function formatMarkdown(text: string): string {
    if (!text) return '<p class="text-slate-500 italic text-xs">No description provided.</p>';
    
    // Escape HTML tags to prevent XSS
    let escaped = text
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
      
    // Code blocks: ```code```
    escaped = escaped.replace(/```([\s\S]*?)```/g, (_, code) => {
      return `<pre class="bg-black/60 border border-slate-900 rounded-lg p-3 my-2.5 font-mono text-[11px] overflow-x-auto text-slate-300 select-text whitespace-pre">${code.trim()}</pre>`;
    });
    
    // Inline code: `code`
    escaped = escaped.replace(/`([^`\n]+)`/g, '<code class="bg-slate-900 border border-slate-800 px-1 py-0.5 rounded font-mono text-[11px] text-indigo-400 select-text">$1</code>');
    
    // Headers: # Header
    escaped = escaped.replace(/^### (.*$)/gim, '<h5 class="text-xs font-bold text-slate-200 mt-4 mb-2 select-text">$1</h5>');
    escaped = escaped.replace(/^## (.*$)/gim, '<h4 class="text-sm font-bold text-white mt-5 mb-2 border-b border-slate-900/50 pb-1 select-text">$1</h4>');
    escaped = escaped.replace(/^# (.*$)/gim, '<h3 class="text-base font-bold text-white mt-6 mb-3 border-b border-slate-900 pb-1.5 select-text">$1</h3>');
    
    // Bold: **bold**
    escaped = escaped.replace(/\*\*([^*]+)\*\*/g, '<strong class="font-bold text-slate-200">$1</strong>');
    
    // Lists: - list item
    escaped = escaped.replace(/^\s*[-*]\s+(.*)$/gim, '<li class="ml-4 list-disc text-slate-300 my-1 select-text">$1</li>');
    
    // Convert newlines to breaks outside pre blocks
    const parts = escaped.split(/(<pre[\s\S]*?<\/pre>)/);
    for (let i = 0; i < parts.length; i++) {
      if (!parts[i].startsWith("<pre")) {
        parts[i] = parts[i].replace(/\n/g, "<br>");
      }
    }
    return parts.join("");
  }

  // Safe formatter for system activity notes (handles HTML and Markdown links)
  function formatSystemNote(text: string): string {
    if (!text) return "";
    
    // Replace markdown links with clickable external links routed through Wails Browser.OpenURL
    let formatted = text.replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_, linkText, url) => {
      return `<a href="javascript:void(0)" onclick="window.__openExternal('${url}')" class="text-indigo-400 hover:underline hover:text-indigo-350 transition duration-150">${linkText}</a>`;
    });
    
    // Format inline code/hashes `c9c77ced`
    formatted = formatted.replace(/`([^`\n]+)`/g, '<code class="bg-slate-900 border border-slate-800 px-1 py-0.5 rounded font-mono text-[10px] text-pink-400 select-text">$1</code>');
    
    return formatted;
  }

  // Load fresh details for the MR
  async function loadMRDetails() {
    if (!mr) return;
    isLoadingMR = true;
    mrError = "";
    try {
      const fresh = await GetSingleMergeRequest(mr.project_id, mr.iid);
      if (fresh) {
        detailedMR = fresh;
      }
    } catch (e: any) {
      console.error("[MRInspectorPanel] failed to load MR details:", e);
      mrError = e.message || "Failed to load MR details.";
    } finally {
      isLoadingMR = false;
    }
  }

  // Load commits for the MR
  async function loadCommits() {
    if (!mr) return;
    isLoadingCommits = true;
    commitsError = "";
    try {
      const fetched = await GetMergeRequestCommits(mr.project_id, mr.iid);
      commits = fetched || [];
    } catch (e: any) {
      console.error("[MRInspectorPanel] failed to load commits:", e);
      commitsError = e.message || "Failed to load commits.";
    } finally {
      isLoadingCommits = false;
    }
  }

  // Load comments/notes for the MR
  async function loadNotes() {
    if (!mr) return;
    isLoadingNotes = true;
    notesError = "";
    try {
      const fetched = await GetMergeRequestNotes(mr.project_id, mr.iid);
      notes = fetched || [];
    } catch (e: any) {
      console.error("[MRInspectorPanel] failed to load notes:", e);
      notesError = e.message || "Failed to load notes/comments.";
    } finally {
      isLoadingNotes = false;
    }
  }

  // Refresh everything for the current MR
  function refreshAll() {
    loadMRDetails();
    loadCommits();
    loadNotes();
  }

  // Handle posting a comment
  async function handlePostComment() {
    if (!mr || !newCommentText.trim() || isPostingComment) return;
    isPostingComment = true;
    try {
      const posted = await CreateMergeRequestNote(mr.project_id, mr.iid, newCommentText);
      if (posted) {
        newCommentText = "";
        // Fetch comments list to ensure we have the fresh thread
        await loadNotes();
      }
    } catch (e: any) {
      console.error("[MRInspectorPanel] failed to post comment:", e);
      alert("Failed to post comment: " + e.message);
    } finally {
      isPostingComment = false;
    }
  }

  // Merge workflow
  async function handleMergeMR() {
    if (!mr || isProcessingAction) return;
    if (!confirm("Are you sure you want to merge this Merge Request?")) return;
    isProcessingAction = "merging";
    try {
      await MergeMergeRequest(mr.project_id, mr.iid);
      refreshAll();
      if (onRefreshList) onRefreshList();
    } catch (e: any) {
      console.error("[MRInspectorPanel] failed to merge MR:", e);
      alert("Failed to merge Merge Request: " + e.message);
    } finally {
      isProcessingAction = null;
    }
  }

  // Close workflow
  async function handleCloseMR() {
    if (!mr || isProcessingAction) return;
    if (!confirm("Are you sure you want to close this Merge Request?")) return;
    isProcessingAction = "closing";
    try {
      await CloseMergeRequest(mr.project_id, mr.iid);
      refreshAll();
      if (onRefreshList) onRefreshList();
    } catch (e: any) {
      console.error("[MRInspectorPanel] failed to close MR:", e);
      alert("Failed to close Merge Request: " + e.message);
    } finally {
      isProcessingAction = null;
    }
  }

  // Keyboard shortcut handler for comments
  function handleTextareaKeyDown(e: KeyboardEvent) {
    if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
      e.preventDefault();
      handlePostComment();
    }
  }

  // React to MR selection changes
  $effect(() => {
    if (isOpen && mr) {
      untrack(() => {
        detailedMR = mr; // Set initial state from props first
        activeTab = "overview";
        newCommentText = "";
        refreshAll();
      });
    }
  });

  const displayMR = $derived(detailedMR || mr);

  // Filter notes into system actions vs user comments if needed,
  // but we can render both sequentially with different layouts.
  const userCommentsCount = $derived(notes.filter(n => !n.system).length);
</script>

<!-- Backdrop overlay -->
{#if isOpen}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    onclick={onClose}
    class="fixed inset-0 bg-black/60 z-40 backdrop-blur-[2.5px] transition duration-200"
  ></div>
{/if}

<!-- Sliding Drawer Panel -->
<div
  class="fixed top-0 right-0 h-screen w-[850px] max-w-[95vw] bg-slate-950/98 border-l border-slate-900 shadow-2xl z-50 transform {isOpen ? 'translate-x-0' : 'translate-x-full'} transition-transform duration-300 ease-in-out flex flex-col"
>
  {#if displayMR}
    <!-- Panel Header -->
    <div class="p-6 border-b border-slate-900/80 flex flex-col shrink-0 relative bg-slate-950/40">
      <div class="flex items-start justify-between">
        <div class="min-w-0 flex-1 pr-6 select-text">
          <div class="flex items-center space-x-2.5 flex-wrap gap-y-1">
            <!-- MR State Badge -->
            {#if displayMR.state === "merged"}
              <span class="px-2 py-0.5 text-[10px] font-bold bg-indigo-500/15 text-indigo-400 border border-indigo-500/25 rounded uppercase tracking-wider">
                Merged
              </span>
            {:else}
              <span class="px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider border rounded {displayMR.state === 'closed' ? 'bg-slate-800/50 text-slate-400 border-slate-700/30' : (displayMR.work_in_progress || displayMR.draft ? 'bg-slate-700/20 text-slate-355 border-slate-600/30' : 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20')}">
                {displayMR.state === 'opened' ? (displayMR.work_in_progress || displayMR.draft ? 'Draft' : 'Open') : displayMR.state}
              </span>
            {/if}

            <span class="text-xs text-slate-500 font-mono font-semibold">!{displayMR.iid}</span>
            <span class="text-xs text-slate-500 select-none">•</span>
            <span class="text-xs text-slate-400 font-mono truncate">{displayMR.web_url.split('/-/merge_requests/')[0].split('/').slice(3).join('/')}</span>
          </div>

          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
          <!-- MR Title -->
          <h2 class="text-lg font-bold text-slate-100 mt-2 leading-snug cursor-text select-text hover:text-indigo-300 transition-colors duration-150" onclick={() => Browser.OpenURL(displayMR.web_url)} title="Open in GitLab">
            {displayMR.title}
          </h2>
          
          <!-- Author & Branches -->
          <div class="flex items-center space-x-2 mt-2 text-xs text-slate-400 flex-wrap gap-y-1">
            <span class="flex items-center space-x-1 shrink-0 select-text">
              {#if displayMR.author?.avatar_url}
                <img src={displayMR.author.avatar_url} alt="" class="w-4 h-4 rounded-full border border-slate-880" />
              {/if}
              <span class="text-slate-300 font-semibold">{displayMR.author?.name}</span>
            </span>
            <span class="text-slate-655 select-none">opened this</span>
            <span class="text-slate-500 font-medium shrink-0">{formatRelativeTime(displayMR.created_at)}</span>
            
            <span class="text-slate-655 select-none px-1">|</span>

            <!-- Branch Source -> Target -->
            <span class="flex items-center space-x-1 bg-slate-900 border border-slate-850 px-1.5 py-0.5 rounded text-[10px] font-mono text-indigo-350 max-w-[180px] truncate" title={displayMR.source_branch}>
              {displayMR.source_branch}
            </span>
            <span class="text-slate-655 font-mono select-none">&rarr;</span>
            <span class="flex items-center space-x-1 bg-slate-900 border border-slate-850 px-1.5 py-0.5 rounded text-[10px] font-mono text-slate-450 max-w-[180px] truncate" title={displayMR.target_branch}>
              {displayMR.target_branch}
            </span>
          </div>
        </div>

        <!-- Right Side Control Buttons -->
        <div class="flex items-center space-x-2 shrink-0 select-none">
          <!-- Refresh Button -->
          <button
            onclick={refreshAll}
            disabled={isLoadingMR}
            aria-label="Refresh Details"
            title="Refresh Details"
            class="w-9 h-9 flex items-center justify-center hover:bg-slate-900 rounded-lg text-slate-500 hover:text-slate-250 transition cursor-pointer"
          >
            <svg class="w-5 h-5 {isLoadingMR ? 'animate-spin text-indigo-400' : ''}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 1121.21 8H17" />
            </svg>
          </button>
          
          <!-- Close Button -->
          <button
            onclick={onClose}
            aria-label="Close Drawer"
            title="Close Drawer"
            class="w-9 h-9 flex items-center justify-center hover:bg-slate-900 rounded-lg text-slate-500 hover:text-slate-255 transition cursor-pointer"
          >
            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>

      <!-- Tab Buttons -->
      <div class="flex space-x-4 mt-6 border-b border-slate-900/60 select-none">
        <button
          onclick={() => (activeTab = "overview")}
          class="pb-2 text-xs font-semibold border-b-2 transition-colors duration-150 {activeTab === 'overview' ? 'border-indigo-500 text-indigo-400' : 'border-transparent text-slate-400 hover:text-slate-200'}"
        >
          Overview
        </button>
        
        <button
          onclick={() => (activeTab = "commits")}
          class="pb-2 text-xs font-semibold border-b-2 transition-colors duration-150 flex items-center space-x-1.5 {activeTab === 'commits' ? 'border-indigo-500 text-indigo-400' : 'border-transparent text-slate-400 hover:text-slate-200'}"
        >
          <span>Commits</span>
          {#if commits.length > 0}
            <span class="px-1.5 py-0.2 bg-slate-900 border border-slate-800 text-[10px] text-slate-400 rounded-full font-mono">{commits.length}</span>
          {/if}
        </button>
        
        <button
          onclick={() => (activeTab = "comments")}
          class="pb-2 text-xs font-semibold border-b-2 transition-colors duration-150 flex items-center space-x-1.5 {activeTab === 'comments' ? 'border-indigo-500 text-indigo-400' : 'border-transparent text-slate-400 hover:text-slate-200'}"
        >
          <span>Comments & Activity</span>
          {#if notes.length > 0}
            <span class="px-1.5 py-0.2 bg-slate-900 border border-slate-800 text-[10px] text-slate-400 rounded-full font-mono">{userCommentsCount}</span>
          {/if}
        </button>
      </div>
    </div>

    <!-- Panel Content Area -->
    <div class="flex-1 overflow-hidden min-h-0 bg-slate-950/20 relative">
      
      <!-- 1. OVERVIEW TAB -->
      {#if activeTab === "overview"}
        <div class="h-full overflow-y-auto p-6 flex flex-col md:flex-row gap-6">
          <!-- Left Content (Description Body) -->
          <div class="flex-1 min-w-0 select-text">
            <h3 class="text-xs font-semibold text-slate-400 uppercase tracking-wider mb-3 select-none">Description</h3>
            <div class="bg-slate-950/40 border border-slate-900/60 rounded-xl p-5 text-sm text-slate-350 leading-relaxed max-w-none overflow-x-auto selection:bg-indigo-500/30 select-text">
              {@html formatMarkdown(displayMR.description)}
            </div>

            <!-- Activity & Discussion Section (Expandable) -->
            <div class="mt-6 flex items-center justify-between border-b border-slate-900/60 pb-3 mb-4 select-none">
              <div class="flex items-center space-x-2">
                <h3 class="text-xs font-semibold text-slate-400 uppercase tracking-wider">Activity & Discussion</h3>
                {#if notes.length > 0}
                  <span class="px-1.5 py-0.2 bg-slate-900 border border-slate-800 text-[10px] text-slate-400 rounded-full font-mono">{userCommentsCount}</span>
                {/if}
              </div>
              <button 
                onclick={() => isActivityExpanded = !isActivityExpanded} 
                class="text-xs text-indigo-400 hover:text-indigo-300 font-semibold cursor-pointer transition duration-150"
              >
                {isActivityExpanded ? 'Hide Activity' : 'Show Activity'}
              </button>
            </div>

            {#if isActivityExpanded}
              <!-- Combined Activity Feed -->
              <div class="space-y-4 mb-6">
                <!-- Activity list -->
                {#if isLoadingNotes}
                  <div class="py-10 flex flex-col items-center justify-center space-y-2">
                    <div class="w-5 h-5 border-2 border-indigo-500 border-t-transparent rounded-full animate-spin"></div>
                    <p class="text-[10px] text-slate-550 font-mono">Loading activity...</p>
                  </div>
                {:else if notesError}
                  <p class="text-xs text-rose-400 font-mono">{notesError}</p>
                {:else if notes.length === 0}
                  <p class="text-xs text-slate-555 italic py-2">No comments or activity logs yet.</p>
                {:else}
                  <div class="space-y-4">
                    {#each notes as note (note.id)}
                      {#if note.system}
                        <!-- System Activity Row -->
                        <div class="flex items-start space-x-2.5 text-xs text-slate-405 px-2 py-0.5 select-text">
                          <span class="w-1.5 h-1.5 rounded-full bg-slate-800 border border-slate-700 mt-1.5 shrink-0"></span>
                          <div class="flex-1 min-w-0">
                            <div class="flex items-baseline space-x-1.5 flex-wrap">
                              <span class="font-semibold text-slate-300">{note.author?.name}</span>
                              <span class="system-note-body leading-relaxed select-text">
                                {@html formatSystemNote(note.body)}
                              </span>
                              <span class="text-[10px] text-slate-505 font-mono whitespace-nowrap">{formatRelativeTime(note.created_at)}</span>
                            </div>
                          </div>
                        </div>
                      {:else}
                        <!-- User Comment Card -->
                        <div class="flex items-start space-x-3 bg-slate-950/40 border border-slate-900/50 rounded-xl p-4 selection:bg-indigo-500/30 select-text">
                          {#if note.author?.avatar_url}
                            <img src={note.author.avatar_url} alt="" class="w-7 h-7 rounded-full shrink-0 border border-slate-855 mt-0.5" />
                          {:else}
                            <div class="w-7 h-7 rounded-full bg-slate-800 shrink-0 border border-slate-700 flex items-center justify-center text-slate-400 text-[10px] font-bold mt-0.5">
                              {note.author?.name?.slice(0, 2).toUpperCase() || 'U'}
                            </div>
                          {/if}
                          <div class="flex-1 min-w-0 select-text">
                            <div class="flex items-baseline justify-between select-none">
                              <div class="flex items-baseline space-x-1.5">
                                <span class="text-xs font-semibold text-slate-200">{note.author?.name}</span>
                                <span class="text-[10px] text-slate-500">@{note.author?.username}</span>
                              </div>
                              <span class="text-[10px] text-slate-505 font-mono">{formatRelativeTime(note.created_at)}</span>
                            </div>
                            <div class="mt-2 text-xs text-slate-355 leading-relaxed max-w-none overflow-x-auto select-text">
                              {@html formatMarkdown(note.body)}
                            </div>
                          </div>
                        </div>
                      {/if}
                    {/each}
                  </div>
                {/if}

                <!-- Quick Comment Input -->
                <div class="mt-4 border border-slate-900 bg-slate-950/50 rounded-xl p-4">
                  <textarea
                    bind:value={newCommentText}
                    onkeydown={handleTextareaKeyDown}
                    placeholder="Write a comment... (Cmd/Ctrl + Enter to send)"
                    rows="2"
                    class="w-full bg-slate-950/80 border border-slate-900 hover:border-slate-850 focus:border-indigo-600 rounded-lg p-3 text-xs text-slate-250 placeholder-slate-655 focus:outline-none resize-none transition select-text"
                  ></textarea>
                  <div class="flex justify-between items-center mt-2.5 select-none">
                    <span class="text-[9px] font-mono text-slate-600">Supports Basic Markdown</span>
                    <button
                      onclick={handlePostComment}
                      disabled={!newCommentText.trim() || isPostingComment}
                      class="px-4 py-1.5 bg-indigo-655 hover:bg-indigo-600 disabled:bg-slate-900 text-white border border-indigo-500/20 disabled:border-transparent rounded-lg text-xs font-semibold shadow-sm transition flex items-center space-x-1.5 cursor-pointer"
                    >
                      {#if isPostingComment}
                        <div class="w-3.5 h-3.5 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
                        <span>Commenting...</span>
                      {:else}
                        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
                        </svg>
                        <span>Comment</span>
                      {/if}
                    </button>
                  </div>
                </div>
              </div>
            {/if}
          </div>

          <!-- Right Content (Metadata Sidebar) -->
          <div class="w-full md:w-60 shrink-0 flex flex-col space-y-6 select-none">
            
            <!-- Workflow Actions -->
            <div class="bg-slate-900/30 border border-slate-900 rounded-xl p-4 flex flex-col space-y-2.5">
              <span class="text-[10px] font-bold text-slate-505 uppercase tracking-wider mb-0.5">Workflow Control</span>
              
              {#if displayMR.state === "opened"}
                <!-- Merge Button -->
                <button
                  onclick={handleMergeMR}
                  disabled={!!isProcessingAction}
                  class="w-full py-2 bg-emerald-650 hover:bg-emerald-600 disabled:bg-slate-800 text-white border border-emerald-500/20 disabled:border-slate-800 rounded-lg text-xs font-semibold shadow-sm transition flex items-center justify-center space-x-2 cursor-pointer"
                >
                  {#if isProcessingAction === "merging"}
                    <div class="w-3.5 h-3.5 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
                    <span>Merging MR...</span>
                  {:else}
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
                    </svg>
                    <span>Accept & Merge</span>
                  {/if}
                </button>

                <!-- Close Button -->
                <button
                  onclick={handleCloseMR}
                  disabled={!!isProcessingAction}
                  class="w-full py-2 bg-slate-900 hover:bg-slate-855 disabled:bg-slate-800 text-rose-450 hover:text-rose-350 border border-slate-800 disabled:border-slate-850 rounded-lg text-xs font-semibold transition flex items-center justify-center space-x-2 cursor-pointer"
                >
                  {#if isProcessingAction === "closing"}
                    <div class="w-3.5 h-3.5 border-2 border-rose-400 border-t-transparent rounded-full animate-spin"></div>
                    <span>Closing MR...</span>
                  {:else}
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    <span>Close Merge Request</span>
                  {/if}
                </button>
              {/if}

              <!-- Open in GitLab -->
              <button
                onclick={() => Browser.OpenURL(displayMR.web_url)}
                class="w-full py-2 bg-slate-900 hover:bg-slate-850 text-slate-300 hover:text-white border border-slate-800 rounded-lg text-xs font-semibold transition flex items-center justify-center space-x-2 cursor-pointer"
              >
                <svg class="w-4 h-4 text-slate-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                </svg>
                <span>View on GitLab</span>
              </button>
            </div>

            <!-- Head Pipeline Status -->
            {#if displayMR.head_pipeline}
              <div>
                <span class="text-[10px] font-bold text-slate-500 uppercase tracking-wider block mb-2">Head Pipeline</span>
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <!-- svelte-ignore a11y_no_static_element_interactions -->
                <div 
                  onclick={() => displayMR.head_pipeline?.web_url && Browser.OpenURL(displayMR.head_pipeline.web_url)}
                  class="flex items-center justify-between p-3 bg-slate-900/30 border border-slate-900 rounded-xl cursor-pointer hover:border-slate-800 transition"
                >
                  <div class="flex items-center space-x-2">
                    <span class="w-2.5 h-2.5 rounded-full {displayMR.head_pipeline.status === 'success' ? 'bg-emerald-500' : (displayMR.head_pipeline.status === 'failed' ? 'bg-rose-500' : (displayMR.head_pipeline.status === 'running' || displayMR.head_pipeline.status === 'pending' ? 'bg-amber-500' : 'bg-slate-505'))}"></span>
                    <span class="text-xs font-mono font-medium text-slate-200 capitalize">{displayMR.head_pipeline.status}</span>
                  </div>
                  <svg class="w-3.5 h-3.5 text-slate-550" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                  </svg>
                </div>
              </div>
            {/if}

            <!-- Assignees -->
            <div>
              <span class="text-[10px] font-bold text-slate-550 uppercase tracking-wider block mb-2">Assignees</span>
              {#if displayMR.assignees && displayMR.assignees.length > 0}
                <div class="space-y-2">
                  {#each displayMR.assignees as assignee (assignee.id)}
                    <div class="flex items-center space-x-2 bg-slate-900/20 border border-slate-900/40 rounded-lg p-2">
                      {#if assignee.avatar_url}
                        <img src={assignee.avatar_url} alt="" class="w-5 h-5 rounded-full" />
                      {/if}
                      <span class="text-xs text-slate-300 font-medium truncate">{assignee.name}</span>
                    </div>
                  {/each}
                </div>
              {:else}
                <span class="text-xs text-slate-550 italic">No assignees</span>
              {/if}
            </div>

            <!-- Reviewers -->
            <div>
              <span class="text-[10px] font-bold text-slate-550 uppercase tracking-wider block mb-2">Reviewers</span>
              {#if displayMR.reviewers && displayMR.reviewers.length > 0}
                <div class="space-y-2">
                  {#each displayMR.reviewers as reviewer (reviewer.id)}
                    <div class="flex items-center space-x-2 bg-slate-900/20 border border-slate-900/40 rounded-lg p-2">
                      {#if reviewer.avatar_url}
                        <img src={reviewer.avatar_url} alt="" class="w-5 h-5 rounded-full" />
                      {/if}
                      <span class="text-xs text-slate-300 font-medium truncate">{reviewer.name}</span>
                    </div>
                  {/each}
                </div>
              {:else}
                <span class="text-xs text-slate-550 italic">No reviewers</span>
              {/if}
            </div>

            <!-- Labels -->
            {#if displayMR.labels && displayMR.labels.length > 0}
              <div>
                <span class="text-[10px] font-bold text-slate-550 uppercase tracking-wider block mb-2">Labels</span>
                <div class="flex flex-wrap gap-1.5">
                  {#each displayMR.labels as label}
                    <span 
                      class="px-2 py-0.5 rounded-md text-[10px] font-medium border"
                      style="color: {getLabelColorHash(label)}; border-color: {getLabelColorHash(label)}30; background-color: {getLabelColorHash(label)}08"
                    >
                      {label}
                    </span>
                  {/each}
                </div>
              </div>
            {/if}
            
          </div>
        </div>
      {/if}

      <!-- 2. COMMITS TAB -->
      {#if activeTab === "commits"}
        <div class="h-full overflow-y-auto p-6 select-text">
          {#if isLoadingCommits}
            <div class="h-60 flex flex-col items-center justify-center space-y-3">
              <div class="w-7 h-7 border-2 border-indigo-500 border-t-transparent rounded-full animate-spin"></div>
              <p class="text-xs text-slate-400 font-mono">Loading commits...</p>
            </div>
          {:else if commitsError}
            <div class="p-4 bg-rose-950/20 border border-rose-900/40 rounded-xl text-rose-450 text-xs font-mono">
              <h4 class="font-bold mb-1">Error loading commits:</h4>
              {commitsError}
              <button 
                onclick={loadCommits}
                class="mt-3 px-3 py-1 bg-rose-900/40 hover:bg-rose-900/60 border border-rose-900 rounded font-semibold text-white transition cursor-pointer"
              >
                Retry
              </button>
            </div>
          {:else if commits.length === 0}
            <div class="h-60 flex flex-col items-center justify-center text-slate-500 text-xs">
              <svg class="w-8 h-8 text-slate-700 mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
              </svg>
              <span>No commits found in this Merge Request.</span>
            </div>
          {:else}
            <div class="space-y-3">
              {#each commits as commit (commit.id)}
                <!-- Commit Item -->
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <!-- svelte-ignore a11y_no_static_element_interactions -->
                <div 
                  onclick={() => commit.web_url && Browser.OpenURL(commit.web_url)}
                  class="group flex items-start justify-between p-4 bg-slate-950/45 border border-slate-900 rounded-xl hover:border-slate-850 cursor-pointer transition"
                >
                  <div class="min-w-0 pr-4">
                    <h4 class="text-xs font-semibold text-slate-200 group-hover:text-indigo-400 transition truncate leading-relaxed">
                      {commit.title}
                    </h4>
                    <div class="flex items-center space-x-2 mt-2 text-[10px] text-slate-500">
                      <span class="text-slate-400 font-medium">{commit.author_name}</span>
                      <span>•</span>
                      <span>{formatRelativeTime(commit.authored_date)}</span>
                    </div>
                  </div>
                  <div class="flex items-center space-x-2 shrink-0">
                    <span class="px-2 py-0.5 bg-slate-900 border border-slate-850 hover:border-slate-750 font-mono text-[10px] text-indigo-350 rounded font-semibold select-all">
                      {commit.short_id}
                    </span>
                    <svg class="w-3.5 h-3.5 text-slate-600 group-hover:text-slate-400 transition" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                    </svg>
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      {/if}

      <!-- 3. COMMENTS TAB -->
      {#if activeTab === "comments"}
        <div class="h-full flex flex-col min-h-0 bg-slate-950/20">
          <!-- Comments List -->
          <div class="flex-1 overflow-y-auto p-6 space-y-4 select-text">
            {#if isLoadingNotes}
              <div class="h-60 flex flex-col items-center justify-center space-y-3">
                <div class="w-7 h-7 border-2 border-indigo-500 border-t-transparent rounded-full animate-spin"></div>
                <p class="text-xs text-slate-400 font-mono">Loading activity and discussions...</p>
              </div>
            {:else if notesError}
              <div class="p-4 bg-rose-950/20 border border-rose-900/40 rounded-xl text-rose-450 text-xs font-mono">
                <h4 class="font-bold mb-1">Error loading comments:</h4>
                {notesError}
                <button 
                  onclick={loadNotes}
                  class="mt-3 px-3 py-1 bg-rose-900/40 hover:bg-rose-900/60 border border-rose-900 rounded font-semibold text-white transition cursor-pointer"
                >
                  Retry
                </button>
              </div>
            {:else if notes.length === 0}
              <div class="h-60 flex flex-col items-center justify-center text-slate-500 text-xs">
                <svg class="w-8 h-8 text-slate-700 mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
                </svg>
                <span>No comments or activity logs yet.</span>
              </div>
            {:else}
              <div class="space-y-4">
                {#each notes as note (note.id)}
                  {#if note.system}
                    <!-- System Activity Row -->
                    <div class="flex items-start space-x-2.5 text-xs text-slate-405 px-2 py-0.5 select-text">
                      <span class="w-1.5 h-1.5 rounded-full bg-slate-800 border border-slate-700 mt-1.5 shrink-0"></span>
                      <div class="flex-1 min-w-0">
                        <div class="flex items-baseline space-x-1.5 flex-wrap">
                          <span class="font-semibold text-slate-300">{note.author?.name}</span>
                          <span class="system-note-body leading-relaxed select-text">
                            {@html formatSystemNote(note.body)}
                          </span>
                          <span class="text-[10px] text-slate-500 font-mono whitespace-nowrap">{formatRelativeTime(note.created_at)}</span>
                        </div>
                      </div>
                    </div>
                  {:else}
                    <!-- User Comment Card -->
                    <div class="flex items-start space-x-3 bg-slate-950/40 border border-slate-900/50 rounded-xl p-4 selection:bg-indigo-500/30 select-text">
                      <!-- Author Avatar -->
                      {#if note.author?.avatar_url}
                        <img src={note.author.avatar_url} alt="" class="w-7 h-7 rounded-full shrink-0 border border-slate-850 mt-0.5" />
                      {:else}
                        <div class="w-7 h-7 rounded-full bg-slate-800 shrink-0 border border-slate-700 flex items-center justify-center text-slate-400 text-[10px] font-bold mt-0.5">
                          {note.author?.name?.slice(0, 2).toUpperCase() || 'U'}
                        </div>
                      {/if}
                      
                      <!-- Comment Content -->
                      <div class="flex-1 min-w-0 select-text">
                        <div class="flex items-baseline justify-between select-none">
                          <div class="flex items-baseline space-x-1.5">
                            <span class="text-xs font-semibold text-slate-200">{note.author?.name}</span>
                            <span class="text-[10px] text-slate-500">@{note.author?.username}</span>
                          </div>
                          <span class="text-[10px] text-slate-500 font-mono">{formatRelativeTime(note.created_at)}</span>
                        </div>
                        <div class="mt-2 text-xs text-slate-350 leading-relaxed max-w-none overflow-x-auto select-text">
                          {@html formatMarkdown(note.body)}
                        </div>
                      </div>
                    </div>
                  {/if}
                {/each}
              </div>
            {/if}
          </div>

          <!-- Comment Composer Footer -->
          <div class="p-4 border-t border-slate-900 bg-slate-950/60 shrink-0">
            <div class="flex flex-col space-y-3">
              <textarea
                bind:value={newCommentText}
                onkeydown={handleTextareaKeyDown}
                placeholder="Write a comment... (Cmd/Ctrl + Enter to send)"
                rows="3"
                class="w-full bg-slate-950/80 border border-slate-900 hover:border-slate-850 focus:border-indigo-600 rounded-lg p-3 text-xs text-slate-250 placeholder-slate-600 focus:outline-none resize-none transition select-text"
              ></textarea>
              <div class="flex justify-between items-center select-none">
                <span class="text-[9px] font-mono text-slate-600">Supports Basic Markdown (**bold**, `code`, ```blocks```)</span>
                <button
                  onclick={handlePostComment}
                  disabled={!newCommentText.trim() || isPostingComment}
                  class="px-4 py-1.5 bg-indigo-655 hover:bg-indigo-600 disabled:bg-slate-900 text-white border border-indigo-500/20 disabled:border-transparent rounded-lg text-xs font-semibold shadow-sm transition flex items-center space-x-1.5 cursor-pointer"
                >
                  {#if isPostingComment}
                    <div class="w-3 h-3 border-2 border-white border-t-transparent rounded-full animate-spin"></div>
                    <span>Commenting...</span>
                  {:else}
                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
                    </svg>
                    <span>Comment</span>
                  {/if}
                </button>
              </div>
            </div>
          </div>
        </div>
      {/if}
      
    </div>
  {:else}
    <!-- Empty State / Missing MR -->
    <div class="flex-1 flex flex-col items-center justify-center p-6 text-slate-550 text-xs">
      <svg class="w-8 h-8 text-slate-700 mb-2 animate-pulse" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
      </svg>
      <span>No Merge Request loaded.</span>
    </div>
  {/if}
</div>

<style>
  /* Custom scrollbar styles for drawer */
  div::-webkit-scrollbar {
    width: 6px;
    height: 6px;
  }
  div::-webkit-scrollbar-track {
    background: transparent;
  }
  div::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.08);
    border-radius: 9999px;
  }
  div::-webkit-scrollbar-thumb:hover {
    background: rgba(255, 255, 255, 0.15);
  }

  :global(.system-note-body ul) {
    margin-top: 4px;
    margin-bottom: 4px;
    padding-left: 18px;
    list-style-type: disc;
  }
  :global(.system-note-body li) {
    margin-top: 2px;
    margin-bottom: 2px;
    font-size: 11px;
    color: #94a3b8; /* slate-400 */
  }
  :global(.system-note-body a) {
    color: #818cf8; /* indigo-400 */
    font-weight: 500;
  }
  :global(.system-note-body a:hover) {
    color: #a5b4fc; /* indigo-350 */
    text-decoration: underline;
  }
</style>
