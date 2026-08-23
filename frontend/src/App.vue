<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ConnectError } from '@connectrpc/connect'
import NotificationPopups from 'picocrank/vue/components/NotificationPopups.vue'
import {
  connectToFirstAvailableController,
  connectWithUserSuppliedBaseUrl,
  persistStoredControllerBaseUrl,
  readStoredControllerBaseUrl,
} from './api/controllerClient'

type Phase = 'loading' | 'prompt' | 'ready'

const phase = ref<Phase>('loading')
const promptUrl = ref('')
const promptErr = ref<string | null>(null)
const lastDiscoveryErr = ref('')
const attempted = ref<string[]>([])

const defaultHint = computed(() =>
  typeof window !== 'undefined'
    ? `${window.location.protocol}//${window.location.hostname}:8080`
    : 'http://127.0.0.1:8080',
)

async function runDiscovery() {
  phase.value = 'loading'
  promptErr.value = null
  const r = await connectToFirstAvailableController()
  if (r.ok) {
    phase.value = 'ready'
    return
  }
  attempted.value = r.attempted
  lastDiscoveryErr.value = r.lastError
  phase.value = 'prompt'
  promptUrl.value = readStoredControllerBaseUrl() ?? defaultHint.value
}

onMounted(() => {
  void runDiscovery()
})

async function submitPrompt() {
  promptErr.value = null
  try {
    await connectWithUserSuppliedBaseUrl(promptUrl.value, true)
    phase.value = 'ready'
  } catch (e) {
    promptErr.value =
      e instanceof ConnectError ? e.message : e instanceof Error ? e.message : String(e)
  }
}

function forgetStoredAndRetry() {
  persistStoredControllerBaseUrl(null)
  void runDiscovery()
}
</script>

<template>
  <div id="app-shell">
    <div v-if="phase === 'loading'" class="connect-gate">
      <p class="connect-gate-title">Connecting to Controller…</p>
    </div>
    <div v-else-if="phase === 'prompt'" class="connect-gate connect-gate-prompt">
      <h1 class="connect-gate-title">Controller not reachable</h1>
      <p class="connect-gate-lead">
        Tried: {{ attempted.join(', ') }}. Last error: {{ lastDiscoveryErr }}
      </p>
      <form class="connect-form" @submit.prevent="submitPrompt">
        <label class="connect-label" for="controller-url">Controller base URL</label>
        <input
          id="controller-url"
          v-model="promptUrl"
          type="text"
          name="controllerUrl"
          class="connect-input"
          autocomplete="url"
          :placeholder="defaultHint"
        />
        <p v-if="promptErr" class="connect-err">{{ promptErr }}</p>
        <div class="connect-actions">
          <button type="submit" class="good">Connect</button>
          <button type="button" class="neutral" @click="forgetStoredAndRetry">Forget saved URL &amp; retry</button>
        </div>
      </form>
    </div>
    <router-view v-else />
    <NotificationPopups />
  </div>
</template>

<style scoped>
.connect-gate {
  min-height: 50vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 2rem 1.25rem;
  color: #0f172a;
}
.connect-gate-prompt {
  align-items: stretch;
  max-width: 36rem;
  margin: 0 auto;
}
.connect-gate-title {
  margin: 0 0 0.5rem;
  font-size: 1.25rem;
  font-weight: 600;
}
.connect-gate-lead {
  margin: 0 0 1.25rem;
  font-size: 0.9rem;
  color: #475569;
  line-height: 1.5;
}
.connect-form {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.connect-label {
  font-size: 0.875rem;
  font-weight: 500;
  color: #334155;
}
.connect-input {
  width: 100%;
  box-sizing: border-box;
  padding: 0.5rem 0.65rem;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  font-size: 0.9375rem;
}
.connect-input:focus {
  outline: 2px solid #0369a1;
  outline-offset: 1px;
}
.connect-err {
  margin: 0;
  font-size: 0.875rem;
  color: #b91c1c;
}
.connect-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-top: 0.35rem;
}
</style>
