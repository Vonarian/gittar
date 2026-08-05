<script lang="ts">
  import { untrack } from "svelte";
  import { GetJobLogSnippet, AnalyzePipelineFailure, type PipelineRCAResult } from "../../../bindings/gittar/internal/service/appservice";

  interface Props {
    isOpen: boolean;
    jobName: string;
    jobId: number;
    projectPath: string;
    onClose: () => void;
  }

  let { isOpen, jobName, jobId, projectPath, onClose }: Props = $props();

  let logSnippet = $state("");
  let isLoading = $state(false);
  let errorMsg = $state("");

  // AI Pipeline Doctor RCA state
  let activeTab = $state<"ai" | "logs">("ai");
  let rcaResult = $state<PipelineRCAResult | null>(null);
  let isAnalyzing = $state(false);
  let rcaError = $state("");
  let copiedFix = $state(false);

  // Clean ANSI colors/codes
  function stripAnsiCodes(text: string): string {
    if (!text) return "";
    return text.replace(
      /[\u001b\u009b][[()#;?]*(?:[0-9]{1,4}(?:;[0-9]{0,4})*)?[0-9A-ORZcf-nqry=><]/g,
      ""
    );
  }

  async function loadLogs() {
    if (jobId <= 0) return;
    isLoading = true;
    errorMsg = "";
    logSnippet = "";
    try {
      const logs = await GetJobLogSnippet(projectPath, jobId);
      logSnippet = stripAnsiCodes(logs);
      if (!logSnippet) {
        logSnippet = "Job output trace was empty.";
      }
    } catch (e: any) {
      errorMsg = e.message || "Failed to load job log trace.";
    } finally {
      isLoading = false;
    }
  }

  async function runPipelineDoctor(force = false) {
    if (jobId <= 0 || !projectPath) return;
    isAnalyzing = true;
    rcaError = "";
    if (force) {
      rcaResult = null;
    }
    try {
      const res = await AnalyzePipelineFailure(projectPath, jobId, jobName, force);
      if (res) {
        rcaResult = res;
      } else {
        rcaError = "No RCA result returned from AI model.";
      }
    } catch (e: any) {
      rcaError = e.message || "Failed to analyze pipeline failure with AI.";
    } finally {
      isAnalyzing = false;
    }
  }

  function copySuggestedFix() {
    if (!rcaResult?.suggested_fix) return;
    navigator.clipboard.writeText(rcaResult.suggested_fix);
    copiedFix = true;
    setTimeout(() => {
      copiedFix = false;
    }, 2000);
  }

  // Reactive log & RCA loading when job changes
  $effect(() => {
    if (isOpen && jobId > 0 && projectPath) {
      untrack(() => {
        rcaResult = null;
        rcaError = "";
        activeTab = "ai";
        loadLogs();
        runPipelineDoctor(false);
      });
    }
  });
</script>

<!-- Backdrop overlay -->
{#if isOpen}
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    onclick={onClose}
    class="fixed inset-0 bg-black/50 z-40 backdrop-blur-[2px] transition duration-200"
  ></div>
{/if}

<!-- Drawer Panel -->
<div
  class="fixed top-0 right-0 h-screen w-[640px] bg-slate-950 border-l border-slate-900 shadow-2xl z-50 transform {isOpen ? 'translate-x-0' : 'translate-x-full'} transition-transform duration-300 ease-in-out flex flex-col"
>
  <!-- Panel Header -->
  <div class="p-5 border-b border-slate-900/60 flex items-center justify-between">
    <div class="min-w-0 pr-6">
      <div class="flex items-center space-x-2">
        <span class="px-2 py-0.5 text-[10px] font-bold bg-rose-500/10 text-rose-400 border border-rose-500/20 rounded">
          FAILED JOB
        </span>
        <h3 class="text-base font-semibold text-slate-100 truncate" title={jobName}>{jobName}</h3>
      </div>
      <p class="text-xs text-slate-500 mt-1 font-mono">Job #{jobId} in {projectPath}</p>
    </div>

    <!-- Close button -->
    <button
      aria-label="Close"
      onclick={onClose}
      class="p-1.5 hover:bg-slate-900 rounded-lg text-slate-500 hover:text-slate-350 transition"
    >
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
      </svg>
    </button>
  </div>

  <!-- Tab Selector -->
  <div class="px-5 pt-3 pb-0 border-b border-slate-900 flex space-x-4 text-xs font-semibold">
    <button
      onclick={() => (activeTab = "ai")}
      class="pb-2.5 flex items-center space-x-1.5 transition border-b-2 {activeTab === 'ai' ? 'border-purple-500 text-purple-400' : 'border-transparent text-slate-400 hover:text-slate-200'}"
    >
      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
      </svg>
      <span>🤖 Pipeline Doctor</span>
      {#if isAnalyzing}
        <span class="w-2 h-2 rounded-full bg-purple-400 animate-ping"></span>
      {/if}
    </button>
    <button
      onclick={() => (activeTab = "logs")}
      class="pb-2.5 flex items-center space-x-1.5 transition border-b-2 {activeTab === 'logs' ? 'border-indigo-500 text-indigo-400' : 'border-transparent text-slate-400 hover:text-slate-200'}"
    >
      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
      </svg>
      <span>Terminal Log Trace</span>
    </button>
  </div>

  <!-- Content Body -->
  <div class="flex-1 p-5 overflow-y-auto flex flex-col min-h-0 bg-slate-950/45">
    {#if activeTab === "ai"}
      <!-- AI Pipeline Doctor View -->
      {#if isAnalyzing}
        <div class="flex-1 flex flex-col items-center justify-center space-y-4 py-12">
          <div class="relative flex items-center justify-center">
            <div class="w-12 h-12 border-2 border-purple-500/30 border-t-purple-500 rounded-full animate-spin"></div>
            <span class="absolute text-base">🤖</span>
          </div>
          <div class="text-center space-y-1">
            <h4 class="text-sm font-semibold text-slate-200">Analyzing Job Failure...</h4>
            <p class="text-xs text-slate-400 font-mono">Parsing log trace and querying AI model</p>
          </div>
        </div>
      {:else if rcaError}
        <div class="p-4 bg-rose-950/20 border border-rose-900/50 rounded-xl text-rose-400 text-xs font-mono space-y-3">
          <div class="flex items-center space-x-2">
            <svg class="w-4 h-4 text-rose-400 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <h4 class="font-bold">AI Analysis Failed</h4>
          </div>
          <p class="text-slate-300">{rcaError}</p>
          <button
            onclick={() => runPipelineDoctor(true)}
            class="px-3 py-1 bg-rose-900/40 hover:bg-rose-900/60 border border-rose-900 rounded font-semibold text-white transition flex items-center space-x-1.5"
          >
            <span>Retry AI Analysis</span>
          </button>
        </div>
      {:else if rcaResult}
        <div class="space-y-4">
          <!-- Glassmorphic Header Card -->
          <div class="p-4 rounded-xl bg-gradient-to-r from-purple-950/40 via-indigo-950/20 to-slate-900/40 border border-purple-800/30 backdrop-blur-sm">
            <div class="flex items-center justify-between mb-2">
              <span class="text-[10px] font-bold tracking-wider uppercase text-purple-400 flex items-center space-x-1">
                <span>🤖 AI DIAGNOSIS</span>
              </span>
              <button
                onclick={() => runPipelineDoctor(true)}
                class="text-[11px] text-purple-400 hover:text-purple-300 font-medium flex items-center space-x-1"
              >
                <span>Re-analyze</span>
              </button>
            </div>
            <p class="text-xs text-slate-300 leading-relaxed font-sans">{rcaResult.root_cause}</p>
          </div>

          <!-- Error Location Snippet -->
          {#if rcaResult.error_snippet}
            <div class="bg-black/60 border border-rose-900/30 rounded-xl p-3.5 space-y-2">
              <span class="text-[10px] font-semibold text-rose-400 uppercase tracking-wider block">📍 Error Line / Stack Trace</span>
              <pre class="text-[11px] font-mono text-rose-200/90 whitespace-pre-wrap overflow-x-auto select-text">{rcaResult.error_snippet}</pre>
            </div>
          {/if}

          <!-- Suggested Fix Card -->
          {#if rcaResult.suggested_fix}
            <div class="bg-slate-900/80 border border-emerald-900/40 rounded-xl p-4 space-y-3">
              <div class="flex items-center justify-between">
                <span class="text-[10px] font-semibold text-emerald-400 uppercase tracking-wider flex items-center space-x-1">
                  <span>💡 Recommended Fix</span>
                </span>
                <button
                  onclick={copySuggestedFix}
                  class="px-2.5 py-1 text-[11px] font-semibold rounded bg-emerald-950/80 border border-emerald-800/60 text-emerald-300 hover:bg-emerald-900/60 transition flex items-center space-x-1"
                >
                  {#if copiedFix}
                    <span class="text-emerald-400">✓ Copied!</span>
                  {:else}
                    <span>Copy Fix</span>
                  {/if}
                </button>
              </div>
              <div class="text-xs text-slate-300 font-mono leading-relaxed whitespace-pre-wrap select-text">{rcaResult.suggested_fix}</div>
            </div>
          {/if}
        </div>
      {:else}
        <div class="flex-1 flex flex-col items-center justify-center space-y-3 text-slate-500 py-12">
          <p class="text-xs">No analysis available for this job.</p>
          <button
            onclick={() => runPipelineDoctor(true)}
            class="px-3 py-1.5 bg-purple-900/30 hover:bg-purple-900/50 border border-purple-800/40 text-purple-300 rounded-lg text-xs font-semibold transition"
          >
            Run AI Doctor
          </button>
        </div>
      {/if}

    {:else}
      <!-- Raw Terminal Logs Output -->
      {#if isLoading}
        <div class="flex-1 flex flex-col items-center justify-center space-y-3">
          <div class="w-8 h-8 border-2 border-indigo-500 border-t-transparent rounded-full animate-spin"></div>
          <p class="text-xs text-slate-400 font-mono">Retrieving GitLab Runner logs...</p>
        </div>
      {:else if errorMsg}
        <div class="p-4 bg-rose-950/20 border border-rose-900/50 rounded-xl text-rose-400 text-xs font-mono">
          <h4 class="font-bold mb-1">Error Loading Output Trace:</h4>
          {errorMsg}
          <button
            onclick={loadLogs}
            class="mt-3 block px-3 py-1 bg-rose-900/40 hover:bg-rose-900/60 border border-rose-900 rounded font-semibold text-white transition"
          >
            Retry Fetch
          </button>
        </div>
      {:else}
        <div class="flex-1 flex flex-col bg-black/60 border border-slate-900 rounded-xl p-4 font-mono text-[11px] leading-relaxed text-slate-300 overflow-auto select-text">
          <div class="flex items-center justify-between border-b border-slate-900 pb-2 mb-3 shrink-0 text-[10px] text-slate-500 uppercase tracking-wider font-semibold select-none">
            <span>Terminal Log Trace Snippet</span>
            <button
              onclick={loadLogs}
              class="text-indigo-400 hover:text-indigo-300 font-bold capitalize"
            >
              Refresh Logs
            </button>
          </div>
          <pre class="flex-1 overflow-auto whitespace-pre-wrap font-mono select-text">{logSnippet}</pre>
        </div>
      {/if}
    {/if}
  </div>

  <!-- Panel Footer Actions -->
  <div class="p-4 border-t border-slate-900/60 flex items-center justify-between bg-slate-950/20">
    <div class="flex items-center space-x-2">
      <button
        onclick={() => (activeTab = activeTab === "ai" ? "logs" : "ai")}
        class="text-xs text-slate-400 hover:text-slate-200 transition font-medium"
      >
        Switch to {activeTab === "ai" ? "Terminal Trace" : "Pipeline Doctor"}
      </button>
    </div>
    <button
      onclick={onClose}
      class="px-4 py-2 border border-slate-800 hover:border-slate-700 text-slate-400 hover:text-slate-200 rounded-lg text-xs font-semibold transition"
    >
      Close Inspector
    </button>
  </div>
</div>

