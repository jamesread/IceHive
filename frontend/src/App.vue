<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { ConnectError } from '@connectrpc/connect'
import FormField from 'picocrank/vue/components/FormField.vue'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import RadioGroup from 'picocrank/vue/components/RadioGroup.vue'
import NotificationPopups from 'picocrank/vue/components/NotificationPopups.vue'
import {
  connectToFirstAvailableController,
  connectWithUserSuppliedBaseUrl,
  controllerStartupWarnings,
  persistStoredControllerBaseUrl,
  readStoredControllerBaseUrls,
} from './api/controllerClient'
import { registerNotificationPopupShow } from './utils/notify'

type Phase = 'loading' | 'prompt' | 'ready'

const phase = ref<Phase>('loading')
const promptUrl = ref('')
const savedUrls = ref<string[]>([])
const promptErr = ref<string | null>(null)
const lastDiscoveryErr = ref('')
const attempted = ref<string[]>([])
const notificationPopupsRef = ref<{ show?: (options?: object) => string | null } | null>(null)

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
  savedUrls.value = readStoredControllerBaseUrls()
  promptUrl.value = savedUrls.value[0] ?? defaultHint.value
}

onMounted(() => {
  void runDiscovery()
  void nextTick(() => {
    const show = notificationPopupsRef.value?.show
    if (typeof show === 'function') {
      registerNotificationPopupShow(show)
    }
  })
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
      <FormLayout @submit.prevent="submitPrompt">
        <FormField
          v-if="savedUrls.length"
          label="Recent controller URLs"
          description="Choose one of the last five controllers saved in this browser."
        >
          <RadioGroup
            v-model="promptUrl"
            name="controllerUrlHistory"
            variant="list"
            aria-label="Recent controller URLs"
            :options="savedUrls"
          />
        </FormField>
        <FormField label="Controller base URL" for="controller-url">
          <input
            id="controller-url"
            v-model="promptUrl"
            type="text"
            name="controllerUrl"
            autocomplete="url"
            :placeholder="defaultHint"
          />
        </FormField>
        <p v-if="promptErr" class="connect-err">{{ promptErr }}</p>
        <template #actions>
          <button type="submit" class="good">Connect</button>
          <button type="button" class="neutral" @click="forgetStoredAndRetry">
            Forget saved URLs &amp; retry
          </button>
        </template>
      </FormLayout>
    </div>
    <template v-else>
      <div v-if="controllerStartupWarnings.length" class="startup-warnings" role="status">
        <p v-for="(warning, index) in controllerStartupWarnings" :key="index">{{ warning }}</p>
      </div>
      <router-view />
    </template>
    <NotificationPopups ref="notificationPopupsRef" />
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
.connect-gate-prompt :deep(.radio-group.radio-list) {
  width: 100%;
  max-width: 100%;
}
.connect-gate-prompt :deep(.radio-group.radio-list label) {
  overflow-wrap: anywhere;
}
.connect-err {
  margin: 0;
  font-size: 0.875rem;
  color: #b91c1c;
}
.startup-warnings {
  margin: 0;
  padding: 0.75rem 1.25rem;
  background: #fef3c7;
  color: #92400e;
  border-bottom: 1px solid #fcd34d;
  font-size: 0.875rem;
  line-height: 1.45;
}
.startup-warnings p {
  margin: 0;
}
.startup-warnings p + p {
  margin-top: 0.35rem;
}
</style>
