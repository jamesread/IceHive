<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { create } from '@bufbuild/protobuf'
import { ConnectError } from '@connectrpc/connect'
import AppHeader from '../components/AppHeader.vue'
import AppFooter from '../components/AppFooter.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import QuickSearch from 'picocrank/vue/components/QuickSearch.vue'
import Section from 'picocrank/vue/components/Section.vue'
import {
  ListConfigRequestSchema,
  SetConfigRequestSchema,
} from '../gen/icehive/v1/controller_pb'
import { getControllerClient } from '../api/controllerClient'
import { notifySuccess } from '../utils/notify'

const GITHUB_TOKEN_KEY = 'github.token'

const router = useRouter()
const existingKeys = ref<Set<string>>(new Set())
const loadingKeys = ref(false)
const creating = ref(false)
const err = ref<string | null>(null)
const newKey = ref('')
const newValue = ref('')

const hasGitHubToken = computed(() => existingKeys.value.has(GITHUB_TOKEN_KEY))

function addGitHubTokenPreset() {
  newKey.value = GITHUB_TOKEN_KEY
  newValue.value = ''
}

async function loadExistingKeys() {
  loadingKeys.value = true
  err.value = null
  try {
    const res = await getControllerClient().listConfig(create(ListConfigRequestSchema, {}))
    existingKeys.value = new Set(res.vars.map((v) => v.key))
  } catch (e) {
    err.value = e instanceof ConnectError ? e.message : String(e)
  } finally {
    loadingKeys.value = false
  }
}

async function createKeyRow() {
  err.value = null
  const key = newKey.value.trim()
  if (key === '') {
    err.value = 'Enter a key name before creating.'
    return
  }
  if (existingKeys.value.has(key)) {
    err.value = `Key "${key}" already exists.`
    return
  }
  creating.value = true
  try {
    await getControllerClient().setConfig(
      create(SetConfigRequestSchema, { key, value: newValue.value }),
    )
    notifySuccess(`Created "${key}".`)
    await router.push({ name: 'config' })
  } catch (e) {
    err.value = e instanceof ConnectError ? e.message : String(e)
  } finally {
    creating.value = false
  }
}

function cancel() {
  void router.push({ name: 'config' })
}

onMounted(() => {
  void loadExistingKeys()
})
</script>

<template>
  <div class="shell">
    <AppHeader>
      <template #toolbar>
        <QuickSearch placeholder="Quick search..." />
      </template>
    </AppHeader>
    <main class="config-main">
      <Section
        title="Create configuration key"
        subtitle="Add a new flattened configuration key and value. Changes are written to controller metadata."
      >
        <p v-if="err" class="err" role="alert">{{ err }}</p>
        <p v-if="!hasGitHubToken && !loadingKeys" class="hint">
          GitHub collection requires a <code class="mono">{{ GITHUB_TOKEN_KEY }}</code> entry.
        </p>
        <FormLayout @submit.prevent="createKeyRow">
          <FormField label="Key" for="create-config-key">
            <input
              id="create-config-key"
              v-model="newKey"
              class="mono"
              type="text"
              required
              placeholder="example: amqp.routing_key_control_events"
              :disabled="creating"
            />
          </FormField>
          <FormField label="Value" for="create-config-value">
            <input
              id="create-config-value"
              v-model="newValue"
              class="mono"
              type="text"
              placeholder="Value"
              :disabled="creating"
            />
          </FormField>
          <template #actions>
            <button type="button" class="neutral" :disabled="creating" @click="cancel">Cancel</button>
            <button
              v-if="!hasGitHubToken"
              type="button"
              class="neutral"
              :disabled="creating || loadingKeys"
              @click="addGitHubTokenPreset"
            >
              Use {{ GITHUB_TOKEN_KEY }}
            </button>
            <button type="submit" class="good" :disabled="creating || loadingKeys">
              {{ creating ? 'Creating…' : 'Create key' }}
            </button>
          </template>
        </FormLayout>
      </Section>
    </main>
    <AppFooter />
  </div>
</template>

<style scoped>
.shell {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}
.config-main {
  flex: 1;
  padding: 1rem 1.5rem 2rem;
}
.err {
  background: #fef2f2;
  color: #b91c1c;
  padding: 0.65rem 0.85rem;
  border-radius: 6px;
  margin: 0 0 1rem;
}
.hint {
  margin: 0 0 1rem;
  font-size: 0.875rem;
  color: #64748b;
  line-height: 1.45;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.85rem;
}
</style>
