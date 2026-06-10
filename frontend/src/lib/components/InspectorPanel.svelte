<script lang="ts">
  import { untrack } from "svelte";
  import { GetJobLogSnippet } from "../../../bindings/gittar/internal/service/appservice";

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
    console.log("[InspectorPanel] loadLogs started for jobId:", jobId, "project:", projectPath);
    isLoading = true;
    errorMsg = "";
    logSnippet = "";
    try {
      const logs = await GetJobLogSnippet(projectPath, jobId);
      logSnippet = stripAnsiCodes(logs);
      if (!logSnippet) {
        logSnippet = "Job output trace was empty.";
      }
      console.log("[InspectorPanel] loadLogs success, log size:", logSnippet.length);
    } catch (e: any) {
      console.error("[InspectorPanel] loadLogs failed:", e);
      errorMsg = e.message || "Failed to load job log trace.";
    } finally {
      isLoading = false;
      console.log("[InspectorPanel] loadLogs complete, isLoading set to false");
    }
  }

  // Reactive log loading when job changes
  $effect(() => {
    console.log("[InspectorPanel] effect triggered:", { isOpen, jobId, projectPath });
    if (isOpen && jobId > 0 && projectPath) {
      untrack(() => {
        loadLogs();
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

  <!-- Console Logs Output -->
  <div class="flex-1 p-5 overflow-y-auto flex flex-col min-h-0 bg-slate-950/45">
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
          <span>Terminal Log Snippet (Last 20 Lines)</span>
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
  </div>

  <!-- Panel Footer Actions -->
  <div class="p-4 border-t border-slate-900/60 flex items-center justify-end bg-slate-950/20">
    <button
      onclick={onClose}
      class="px-4 py-2 border border-slate-800 hover:border-slate-700 text-slate-400 hover:text-slate-200 rounded-lg text-xs font-semibold transition"
    >
      Close Inspector
    </button>
  </div>
</div>
