<script lang="ts">
  import { onMount } from "svelte";
  import { GetConfig, SaveConfig, FetchTelemetry, TriggerTestNotification } from "../../../bindings/gittar/internal/service/appservice";
  import { Config } from "../../../bindings/gittar/internal/config/models";

  interface Props {
    onConfigSaved: () => void;
  }

  let { onConfigSaved }: Props = $props();

  let gitlabUrl = $state("https://gitlab.com");
  let token = $state("");
  let monitoredProjectsRaw = $state("");
  let monitoredGroupsRaw = $state("");
  let pollIntervalSec = $state(30);
  let proxyEnabled = $state(false);
  let proxyHost = $state("");
  let proxyPort = $state(1080);
  let proxyUser = $state("");
  let proxyPassword = $state("");
  let aiProvider = $state("openrouter");
  let openrouterApiKey = $state("");
  let openrouterModel = $state("google/gemini-2.5-flash");
  let openaiApiKey = $state("");
  let openaiModel = $state("gpt-4o-mini");
  let openaiBaseUrl = $state("http://localhost:11434/v1");
  let aiCostPreset = $state("low");

  // Notifications fine-tuned preferences
  let notifEnabled = $state(true);
  let notifPipelineSuccess = $state(true);
  let notifPipelineFailed = $state(true);
  let notifMRAssigned = $state(true);
  let notifMRReviewRequest = $state(true);
  let notifTodoMention = $state(true);
  let notifTodoAssignment = $state(true);
  let notifTodoIssue = $state(true);
  let notifTodoGeneric = $state(true);

  let isSaving = $state(false);
  let isTesting = $state(false);
  let testStatus = $state<"idle" | "success" | "error">("idle");
  let testError = $state("");
  let saveSuccess = $state(false);

  let testNotifStatus = $state<"idle" | "success" | "error">("idle");
  let testNotifError = $state("");

  onMount(async () => {
    try {
      const cfg = await GetConfig();
      if (cfg) {
        gitlabUrl = cfg.gitlabUrl || "https://gitlab.com";
        token = cfg.token || "";
        monitoredProjectsRaw = (cfg.monitoredProjects || []).join("\n");
        monitoredGroupsRaw = (cfg.monitoredGroups || []).join("\n");
        pollIntervalSec = cfg.pollIntervalSec || 30;
        proxyEnabled = cfg.proxyEnabled ?? false;
        proxyHost = cfg.proxyHost || "";
        proxyPort = cfg.proxyPort || 1080;
        proxyUser = cfg.proxyUser || "";
        proxyPassword = cfg.proxyPassword || "";
        aiProvider = cfg.aiProvider || "openrouter";
        openrouterApiKey = cfg.openrouterApiKey || "";
        openrouterModel = cfg.openrouterModel || "google/gemini-2.5-flash";
        openaiApiKey = cfg.openaiApiKey || "";
        openaiModel = cfg.openaiModel || "gpt-4o-mini";
        openaiBaseUrl = cfg.openaiBaseUrl || "http://localhost:11434/v1";
        aiCostPreset = cfg.aiCostPreset || "low";

        if (cfg.notifications) {
          notifEnabled = cfg.notifications.enabled ?? true;
          notifPipelineSuccess = cfg.notifications.pipelineSuccess ?? true;
          notifPipelineFailed = cfg.notifications.pipelineFailed ?? true;
          notifMRAssigned = cfg.notifications.mrAssigned ?? true;
          notifMRReviewRequest = cfg.notifications.mrReviewRequest ?? true;
          notifTodoMention = cfg.notifications.todoMention ?? true;
          notifTodoAssignment = cfg.notifications.todoAssignment ?? true;
          notifTodoIssue = cfg.notifications.todoIssue ?? true;
          notifTodoGeneric = cfg.notifications.todoGeneric ?? true;
        }
      }
    } catch (e) {
      console.error("Failed to load config:", e);
    }
  });

  function parseTextarea(raw: string): string[] {
    return raw
      .split("\n")
      .map((line) => line.trim())
      .filter((line) => line.length > 0);
  }

  function toggleAllNotifications(val: boolean) {
    notifPipelineSuccess = val;
    notifPipelineFailed = val;
    notifMRAssigned = val;
    notifMRReviewRequest = val;
    notifTodoMention = val;
    notifTodoAssignment = val;
    notifTodoIssue = val;
    notifTodoGeneric = val;
  }

  async function handleSave() {
    isSaving = true;
    saveSuccess = false;
    try {
      const conf = new Config({
        gitlabUrl: gitlabUrl.trim(),
        token: token.trim(),
        monitoredProjects: parseTextarea(monitoredProjectsRaw),
        monitoredGroups: parseTextarea(monitoredGroupsRaw),
        pollIntervalSec: pollIntervalSec,
        proxyEnabled: proxyEnabled,
        proxyHost: proxyHost.trim(),
        proxyPort: proxyPort,
        proxyUser: proxyUser.trim(),
        proxyPassword: proxyPassword,
        aiProvider: aiProvider,
        openrouterApiKey: openrouterApiKey.trim(),
        openrouterModel: openrouterModel.trim(),
        openaiApiKey: openaiApiKey.trim(),
        openaiModel: openaiModel.trim(),
        openaiBaseUrl: openaiBaseUrl.trim(),
        aiCostPreset: aiCostPreset,
        notifications: {
          enabled: notifEnabled,
          pipelineSuccess: notifPipelineSuccess,
          pipelineFailed: notifPipelineFailed,
          mrAssigned: notifMRAssigned,
          mrReviewRequest: notifMRReviewRequest,
          todoMention: notifTodoMention,
          todoAssignment: notifTodoAssignment,
          todoIssue: notifTodoIssue,
          todoGeneric: notifTodoGeneric,
        },
      });
      await SaveConfig(conf);
      saveSuccess = true;
      onConfigSaved();
      setTimeout(() => {
        saveSuccess = false;
      }, 3000);
    } catch (e: any) {
      console.error(e);
      alert("Failed to save configuration: " + e.message);
    } finally {
      isSaving = false;
    }
  }

  async function handleTest() {
    isTesting = true;
    testStatus = "idle";
    testError = "";
    try {
      const tempConf = new Config({
        gitlabUrl: gitlabUrl.trim(),
        token: token.trim(),
        monitoredProjects: parseTextarea(monitoredProjectsRaw),
        monitoredGroups: parseTextarea(monitoredGroupsRaw),
        pollIntervalSec: pollIntervalSec,
        proxyEnabled: proxyEnabled,
        proxyHost: proxyHost.trim(),
        proxyPort: proxyPort,
        proxyUser: proxyUser.trim(),
        proxyPassword: proxyPassword,
        aiProvider: aiProvider,
        openrouterApiKey: openrouterApiKey.trim(),
        openrouterModel: openrouterModel.trim(),
        openaiApiKey: openaiApiKey.trim(),
        openaiModel: openaiModel.trim(),
        openaiBaseUrl: openaiBaseUrl.trim(),
        aiCostPreset: aiCostPreset,
        notifications: {
          enabled: notifEnabled,
          pipelineSuccess: notifPipelineSuccess,
          pipelineFailed: notifPipelineFailed,
          mrAssigned: notifMRAssigned,
          mrReviewRequest: notifMRReviewRequest,
          todoMention: notifTodoMention,
          todoAssignment: notifTodoAssignment,
          todoIssue: notifTodoIssue,
          todoGeneric: notifTodoGeneric,
        },
      });
      await SaveConfig(tempConf);
      
      const payload = await FetchTelemetry();
      if (payload && !payload.error) {
        testStatus = "success";
      } else {
        testStatus = "error";
        testError = payload?.error || "Connection failed. Please check your URL and Token.";
      }
    } catch (e: any) {
      testStatus = "error";
      testError = e.message || "Request failed.";
    } finally {
      isTesting = false;
    }
  }

  async function handleTestNotification() {
    testNotifStatus = "idle";
    testNotifError = "";
    try {
      await TriggerTestNotification();
      testNotifStatus = "success";
      // Reset success status after 3 seconds
      setTimeout(() => {
        if (testNotifStatus === "success") {
          testNotifStatus = "idle";
        }
      }, 3000);
    } catch (e: any) {
      testNotifStatus = "error";
      testNotifError = e.message || "Failed to trigger notification.";
    }
  }
</script>

<div class="max-w-2xl mx-auto p-6 bg-slate-900/50 border border-slate-800 rounded-xl backdrop-blur-md shadow-2xl space-y-6">
  <div class="flex items-center space-x-4 pb-4 border-b border-slate-800/60">
    <img src="/gittar_logo.png" alt="Gittar Logo" class="w-14 h-14 rounded-xl border border-slate-800/80 shadow-lg shadow-black/40 object-cover" />
    <div>
      <h2 class="text-xl font-bold text-white tracking-tight">Connection Settings</h2>
      <p class="text-slate-450 text-xs mt-0.5">Configure Gittar to connect to GitLab.com or a Self-Managed instance.</p>
    </div>
  </div>

  <div class="space-y-5">
    <!-- GitLab Server URL -->
    <div>
      <label for="gitlab-url" class="block text-sm font-medium text-slate-300 mb-1.5">GitLab Server URL</label>
      <input
        id="gitlab-url"
        type="text"
        bind:value={gitlabUrl}
        placeholder="https://gitlab.com"
        class="w-full px-3 py-2 bg-slate-950/70 border border-slate-800 focus:border-indigo-500 rounded-lg text-slate-200 outline-none transition"
      />
    </div>

    <!-- Personal Access Token -->
    <div>
      <label for="pat-token" class="block text-sm font-medium text-slate-300 mb-1.5">Personal Access Token (PAT)</label>
      <input
        id="pat-token"
        type="password"
        bind:value={token}
        placeholder="glpat-..."
        class="w-full px-3 py-2 bg-slate-950/70 border border-slate-800 focus:border-indigo-500 rounded-lg text-slate-200 outline-none transition font-mono"
      />
      <p class="text-slate-500 text-xs mt-1">Token requires <code>api</code> or <code>read_api</code> scope access.</p>
    </div>

    <div class="grid grid-cols-2 gap-4">
      <!-- Monitored Projects -->
      <div>
        <label for="monitored-projects" class="block text-sm font-medium text-slate-300 mb-1.5">Monitored Projects</label>
        <textarea
          id="monitored-projects"
          bind:value={monitoredProjectsRaw}
          placeholder="group/project-name&#10;another-group/subgroup/project"
          rows="5"
          class="w-full px-3 py-2 bg-slate-950/70 border border-slate-800 focus:border-indigo-500 rounded-lg text-slate-200 outline-none transition font-mono text-xs resize-none"
        ></textarea>
        <p class="text-slate-500 text-xs mt-1">One project path per line.</p>
      </div>

      <!-- Monitored Groups -->
      <div>
        <label for="monitored-groups" class="block text-sm font-medium text-slate-300 mb-1.5">Monitored Groups</label>
        <textarea
          id="monitored-groups"
          bind:value={monitoredGroupsRaw}
          placeholder="corporate-group&#10;another-group/subgroup"
          rows="5"
          class="w-full px-3 py-2 bg-slate-950/70 border border-slate-800 focus:border-indigo-500 rounded-lg text-slate-200 outline-none transition font-mono text-xs resize-none"
        ></textarea>
        <p class="text-slate-500 text-xs mt-1">All projects inside these groups will be tracked.</p>
      </div>
    </div>

    <!-- Polling Interval -->
    <div>
      <label for="poll-interval" class="block text-sm font-medium text-slate-300 mb-1.5">Telemetry Polling Interval (seconds)</label>
      <input
        id="poll-interval"
        type="number"
        min="10"
        max="300"
        bind:value={pollIntervalSec}
        class="w-32 px-3 py-2 bg-slate-950/70 border border-slate-800 focus:border-indigo-500 rounded-lg text-slate-200 outline-none transition"
      />
    </div>

    <!-- SOCKS5 Proxy Settings -->
    <div class="bg-slate-950/40 border border-slate-800/80 rounded-xl p-4 space-y-4">
      <div class="flex items-center justify-between">
        <div>
          <h3 class="text-sm font-bold text-white">SOCKS5 Proxy</h3>
          <p class="text-slate-500 text-xs mt-0.5">Route connection to GitLab server through a SOCKS5 proxy.</p>
        </div>
        <label class="relative inline-flex items-center cursor-pointer select-none">
          <input type="checkbox" bind:checked={proxyEnabled} class="sr-only peer" />
          <div class="w-9 h-5 bg-slate-800 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-slate-400 after:border-slate-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-indigo-600 peer-checked:after:bg-white"></div>
        </label>
      </div>

      {#if proxyEnabled}
        <div class="border-t border-slate-900 pt-3 space-y-4">
          <div class="grid grid-cols-3 gap-4">
            <!-- Proxy Host -->
            <div class="col-span-2">
              <label for="proxy-host" class="block text-xs font-medium text-slate-400 mb-1">Proxy Host</label>
              <input
                id="proxy-host"
                type="text"
                bind:value={proxyHost}
                placeholder="127.0.0.1 or proxy.local"
                class="w-full px-3 py-1.5 bg-slate-950/70 border border-slate-800 focus:border-indigo-500 rounded-lg text-slate-200 outline-none text-xs transition"
              />
            </div>

            <!-- Proxy Port -->
            <div>
              <label for="proxy-port" class="block text-xs font-medium text-slate-400 mb-1">Proxy Port</label>
              <input
                id="proxy-port"
                type="number"
                min="1"
                max="65535"
                bind:value={proxyPort}
                placeholder="1080"
                class="w-full px-3 py-1.5 bg-slate-950/70 border border-slate-800 focus:border-indigo-500 rounded-lg text-slate-200 outline-none text-xs transition"
              />
            </div>
          </div>

          <div class="grid grid-cols-2 gap-4">
            <!-- Proxy User -->
            <div>
              <label for="proxy-user" class="block text-xs font-medium text-slate-400 mb-1">Username (Optional)</label>
              <input
                id="proxy-user"
                type="text"
                bind:value={proxyUser}
                placeholder="username"
                class="w-full px-3 py-1.5 bg-slate-950/70 border border-slate-800 focus:border-indigo-500 rounded-lg text-slate-200 outline-none text-xs transition"
              />
            </div>

            <!-- Proxy Password -->
            <div>
              <label for="proxy-password" class="block text-xs font-medium text-slate-400 mb-1">Password (Optional)</label>
              <input
                id="proxy-password"
                type="password"
                bind:value={proxyPassword}
                placeholder="password"
                class="w-full px-3 py-1.5 bg-slate-950/70 border border-slate-800 focus:border-indigo-500 rounded-lg text-slate-200 outline-none text-xs transition"
              />
            </div>
          </div>
        </div>
      {/if}
    </div>

    <!-- Fine-Tuned Notifications Settings -->
    <div class="bg-slate-950/40 border border-slate-800/80 rounded-xl p-4 space-y-4">
      <div class="flex items-center justify-between">
        <div>
          <h3 class="text-sm font-bold text-white">Desktop Notifications</h3>
          <p class="text-slate-500 text-xs mt-0.5">Configure when Gittar should alert you via system popups.</p>
        </div>
        <label class="relative inline-flex items-center cursor-pointer select-none">
          <input type="checkbox" bind:checked={notifEnabled} class="sr-only peer" />
          <div class="w-9 h-5 bg-slate-800 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-slate-400 after:border-slate-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-indigo-600 peer-checked:after:bg-white"></div>
        </label>
      </div>

      {#if notifEnabled}
        <div class="border-t border-slate-900 pt-3 space-y-3">
          <!-- Multi Select Helpers -->
          <div class="flex items-center space-x-2 text-[10px] uppercase font-bold text-slate-500">
            <span>Quick Actions:</span>
            <button onclick={() => toggleAllNotifications(true)} class="text-indigo-400 hover:text-indigo-300 transition">Select All</button>
            <span>•</span>
            <button onclick={() => toggleAllNotifications(false)} class="text-slate-400 hover:text-slate-300 transition">Clear All</button>
          </div>

          <!-- Notification Toggles Matrix -->
          <div class="grid grid-cols-2 gap-3 pt-1">
            <!-- Pipeline Success -->
            <label class="flex items-center space-x-2.5 text-xs text-slate-300 cursor-pointer select-none">
              <input type="checkbox" bind:checked={notifPipelineSuccess} class="rounded border-slate-800 bg-slate-950 text-indigo-600 focus:ring-0 focus:ring-offset-0" />
              <span>Pipeline Passed alerts</span>
            </label>

            <!-- Pipeline Failed -->
            <label class="flex items-center space-x-2.5 text-xs text-slate-300 cursor-pointer select-none">
              <input type="checkbox" bind:checked={notifPipelineFailed} class="rounded border-slate-800 bg-slate-950 text-indigo-600 focus:ring-0 focus:ring-offset-0" />
              <span>Pipeline Failed alerts</span>
            </label>

            <!-- MR Assigned -->
            <label class="flex items-center space-x-2.5 text-xs text-slate-300 cursor-pointer select-none">
              <input type="checkbox" bind:checked={notifMRAssigned} class="rounded border-slate-800 bg-slate-950 text-indigo-600 focus:ring-0 focus:ring-offset-0" />
              <span>MR Assigned to me</span>
            </label>

            <!-- MR Review requested -->
            <label class="flex items-center space-x-2.5 text-xs text-slate-300 cursor-pointer select-none">
              <input type="checkbox" bind:checked={notifMRReviewRequest} class="rounded border-slate-800 bg-slate-950 text-indigo-600 focus:ring-0 focus:ring-offset-0" />
              <span>MR Review requests</span>
            </label>

            <!-- Todo Mention -->
            <label class="flex items-center space-x-2.5 text-xs text-slate-300 cursor-pointer select-none">
              <input type="checkbox" bind:checked={notifTodoMention} class="rounded border-slate-800 bg-slate-950 text-indigo-600 focus:ring-0 focus:ring-offset-0" />
              <span>Comment @mentions</span>
            </label>

            <!-- Todo Assignment -->
            <label class="flex items-center space-x-2.5 text-xs text-slate-300 cursor-pointer select-none">
              <input type="checkbox" bind:checked={notifTodoAssignment} class="rounded border-slate-800 bg-slate-950 text-indigo-600 focus:ring-0 focus:ring-offset-0" />
              <span>Todo assignments</span>
            </label>

            <!-- Todo Issue -->
            <label class="flex items-center space-x-2.5 text-xs text-slate-300 cursor-pointer select-none">
              <input type="checkbox" bind:checked={notifTodoIssue} class="rounded border-slate-800 bg-slate-950 text-indigo-600 focus:ring-0 focus:ring-offset-0" />
              <span>New Issue todos</span>
            </label>

            <!-- Todo Generic -->
            <label class="flex items-center space-x-2.5 text-xs text-slate-300 cursor-pointer select-none">
              <input type="checkbox" bind:checked={notifTodoGeneric} class="rounded border-slate-800 bg-slate-950 text-indigo-600 focus:ring-0 focus:ring-offset-0" />
              <span>Generic system todos</span>
            </label>
          </div>

          <!-- Send Test Notification -->
          <div class="flex items-center justify-between border-t border-slate-900/60 pt-3">
            <button
              type="button"
              onclick={handleTestNotification}
              disabled={isTesting || isSaving}
              class="px-3 py-1.5 text-xs font-semibold rounded-lg bg-slate-800 text-slate-200 hover:bg-slate-700 transition flex items-center space-x-1.5 disabled:opacity-50"
            >
              <span>Send Test Notification</span>
            </button>
            
            {#if testNotifStatus === "success"}
              <span class="text-emerald-400 text-xs font-medium">✓ Notification triggered! Check your desktop.</span>
            {:else if testNotifStatus === "error"}
              <span class="text-rose-400 text-xs font-medium">✗ {testNotifError}</span>
            {/if}
          </div>
        </div>
      {/if}
    </div>

    <!-- AI Agent & Summaries Settings -->
    <div class="bg-slate-950/40 border border-slate-800/80 rounded-xl p-4 space-y-4">
      <div class="flex items-center justify-between">
        <div>
          <h3 class="text-sm font-bold text-white">AI Agent & Summaries</h3>
          <p class="text-slate-500 text-xs mt-0.5">Enable agentic features and Merge Request summaries by selecting a provider.</p>
        </div>
        <select
          bind:value={aiProvider}
          class="px-2.5 py-1 bg-slate-955/70 border border-slate-800 focus:border-indigo-500 rounded-lg text-slate-200 outline-none text-xs transition select-none"
        >
          <option value="openrouter">OpenRouter</option>
          <option value="openai">OpenAI-Compatible</option>
        </select>
      </div>

      <div class="border-t border-slate-900 pt-3 space-y-4">
        {#if aiProvider === "openrouter"}
          <div class="grid grid-cols-3 gap-4">
            <!-- OpenRouter API Key -->
            <div class="col-span-2">
              <label for="openrouter-apikey" class="block text-xs font-medium text-slate-400 mb-1">OpenRouter API Key</label>
              <input
                id="openrouter-apikey"
                type="password"
                bind:value={openrouterApiKey}
                placeholder="sk-or-v1-..."
                class="w-full px-3 py-1.5 bg-slate-950/70 border border-slate-800 focus:border-indigo-500 rounded-lg text-slate-200 outline-none text-xs transition font-mono"
              />
            </div>

            <!-- OpenRouter Model -->
            <div>
              <label for="openrouter-model" class="block text-xs font-medium text-slate-400 mb-1">OpenRouter Model</label>
              <input
                id="openrouter-model"
                type="text"
                bind:value={openrouterModel}
                placeholder="google/gemini-2.5-flash"
                class="w-full px-3 py-1.5 bg-slate-950/70 border border-slate-800 focus:border-indigo-500 rounded-lg text-slate-200 outline-none text-xs transition font-mono"
              />
            </div>
          </div>
        {:else if aiProvider === "openai"}
          <div class="space-y-4">
            <div class="grid grid-cols-3 gap-4">
              <!-- OpenAI Custom Base URL -->
              <div class="col-span-3">
                <label for="openai-baseurl" class="block text-xs font-medium text-slate-400 mb-1">Base URL</label>
                <input
                  id="openai-baseurl"
                  type="text"
                  bind:value={openaiBaseUrl}
                  placeholder="http://localhost:11434/v1"
                  class="w-full px-3 py-1.5 bg-slate-950/70 border border-slate-800 focus:border-indigo-500 rounded-lg text-slate-200 outline-none text-xs transition font-mono"
                />
                <p class="text-slate-500 text-[10px] mt-1">E.g., <code>http://localhost:11434/v1</code> for Ollama, or <code>https://api.openai.com/v1</code></p>
              </div>
            </div>

            <div class="grid grid-cols-3 gap-4">
              <!-- OpenAI API Key -->
              <div class="col-span-2">
                <label for="openai-apikey" class="block text-xs font-medium text-slate-400 mb-1">API Key (Optional)</label>
                <input
                  id="openai-apikey"
                  type="password"
                  bind:value={openaiApiKey}
                  placeholder="Omit for local providers like Ollama"
                  class="w-full px-3 py-1.5 bg-slate-950/70 border border-slate-800 focus:border-indigo-500 rounded-lg text-slate-200 outline-none text-xs transition font-mono"
                />
              </div>

              <!-- OpenAI Model -->
              <div>
                <label for="openai-model" class="block text-xs font-medium text-slate-400 mb-1">Model Name</label>
                <input
                  id="openai-model"
                  type="text"
                  bind:value={openaiModel}
                  placeholder="gpt-4o-mini or llama3"
                  class="w-full px-3 py-1.5 bg-slate-950/70 border border-slate-800 focus:border-indigo-500 rounded-lg text-slate-200 outline-none text-xs transition font-mono"
                />
              </div>
            </div>
          </div>
        {/if}

        <!-- AI Cost Preset -->
        <div class="border-t border-slate-900 pt-3">
          <label for="ai-cost-preset" class="block text-xs font-medium text-slate-400 mb-1">Context Size / Cost Preset</label>
          <div class="flex items-center space-x-3">
            <select
              id="ai-cost-preset"
              bind:value={aiCostPreset}
              class="px-2.5 py-1.5 bg-slate-950/70 border border-slate-800 focus:border-indigo-500 rounded-lg text-slate-200 outline-none text-xs transition select-none"
            >
              <option value="low">Low (Minimum Context / Cost)</option>
              <option value="medium">Medium (Balanced)</option>
              <option value="high">High (Rich Context)</option>
            </select>
            <p class="text-slate-500 text-[10px]">
              Controls the volume of diffs, description, and commits sent to the AI model.
            </p>
          </div>
        </div>
      </div>
    </div>

    <hr class="border-slate-800/80 my-2" />

    <!-- Actions & Alerts -->
    <div class="flex items-center justify-between pt-2">
      <div class="flex items-center space-x-3">
        <button
          type="button"
          onclick={handleTest}
          disabled={isTesting || isSaving}
          class="px-4 py-2 border border-slate-700 hover:border-slate-600 active:bg-slate-800 text-slate-300 rounded-lg text-sm transition disabled:opacity-50"
        >
          {isTesting ? "Testing..." : "Test Connection"}
        </button>

        {#if testStatus === "success"}
          <span class="text-emerald-500 text-sm font-medium flex items-center">
            <span class="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-ping mr-2"></span>
            Connection Successful
          </span>
        {:else if testStatus === "error"}
          <span class="text-red-500 text-xs max-w-[200px] truncate" title={testError}>
            Connection Failed: {testError}
          </span>
        {/if}
      </div>

      <div class="flex items-center space-x-3">
        {#if saveSuccess}
          <span class="text-emerald-400 text-sm transition">Saved!</span>
        {/if}

        <button
          type="button"
          onclick={handleSave}
          disabled={isSaving || isTesting || !token}
          class="px-5 py-2 bg-indigo-600 hover:bg-indigo-500 active:bg-indigo-700 text-white rounded-lg text-sm font-medium transition disabled:opacity-50 shadow-lg shadow-indigo-600/10"
        >
          {isSaving ? "Saving..." : "Save Settings"}
        </button>
      </div>
    </div>
  </div>
</div>
