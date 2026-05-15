<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import { create } from '@bufbuild/protobuf'
import { ConnectError } from '@connectrpc/connect'
import Header from 'picocrank/vue/components/Header.vue'
import QuickSearch from 'picocrank/vue/components/QuickSearch.vue'
import Section from 'picocrank/vue/components/Section.vue'
import type { ConfigVar } from '../gen/icehive/v1/controller_pb'
import {
  ListConfigRequestSchema,
  SetConfigRequestSchema,
} from '../gen/icehive/v1/controller_pb'
import { getControllerClient } from '../api/controllerClient'

const rows = ref<ConfigVar[]>([])
const edits = ref<Record<string, string>>({})
const loading = ref(false)
const err = ref<string | null>(null)
const savingKey = ref<string | null>(null)
const creating = ref(false)
const newKey = ref('')
const newValue = ref('')

async function loadConfig() {
  err.value = null
  loading.value = true
  try {
    const res = await getControllerClient().listConfig(create(ListConfigRequestSchema, {}))
    rows.value = [...res.vars]
    const next: Record<string, string> = {}
    for (const v of res.vars) {
      next[v.key] = v.redacted ? '' : v.value
    }
    edits.value = next
  } catch (e) {
    err.value = e instanceof ConnectError ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

async function saveRow(key: string) {
  err.value = null
  const v = rows.value.find((r) => r.key === key)
  if (!v) return
  const nextVal = edits.value[key] ?? ''
  if (v.redacted && nextVal === '') {
    err.value = `Enter a new value for redacted key "${key}" before saving.`
    return
  }
  savingKey.value = key
  try {
    await getControllerClient().setConfig(
      create(SetConfigRequestSchema, { key, value: nextVal }),
    )
    await loadConfig()
  } catch (e) {
    err.value = e instanceof ConnectError ? e.message : String(e)
  } finally {
    savingKey.value = null
  }
}

async function createKeyRow() {
  err.value = null
  const key = newKey.value.trim()
  if (key === '') {
    err.value = 'Enter a key name before creating.'
    return
  }
  if (rows.value.some((r) => r.key === key)) {
    err.value = `Key "${key}" already exists.`
    return
  }
  creating.value = true
  try {
    await getControllerClient().setConfig(
      create(SetConfigRequestSchema, { key, value: newValue.value }),
    )
    newKey.value = ''
    newValue.value = ''
    await loadConfig()
  } catch (e) {
    err.value = e instanceof ConnectError ? e.message : String(e)
  } finally {
    creating.value = false
  }
}

onMounted(() => {
  void loadConfig()
})
</script>

<template>
  <div class="shell">
    <Header
      title="IceHive"
      username="Guest"
      :sidebar-enabled="false"
      :show-branding="true"
      logo-url="/favicon.svg"
    >
      <template #toolbar>
        <QuickSearch placeholder="Quick search..." />
      </template>
    </Header>
    <main class="config-main">
      <nav class="crumb">
        <RouterLink to="/">Home</RouterLink>
        <span class="sep">/</span>
        <span>Controller configuration</span>
      </nav>
      <Section title="Controller configuration">
        <p class="lede">
          Values come from the controller process via Connect RPC (<code>ListConfig</code> /
          <code>SetConfig</code>). Changes are written to controller metadata.
        </p>
        <p v-if="err" class="err" role="alert">{{ err }}</p>
        <section class="create-panel">
          <h2>Create configuration key</h2>
          <form class="create-form" @submit.prevent="createKeyRow">
            <label class="field">
              <span>Key</span>
              <input
                v-model="newKey"
                class="val-input mono"
                type="text"
                placeholder="example: amqp.routing_key_control_events"
                aria-label="New key"
              />
            </label>
            <label class="field">
              <span>Value</span>
              <input
                v-model="newValue"
                class="val-input mono"
                type="text"
                placeholder="Value"
                aria-label="New value"
              />
            </label>
            <button type="submit" class="good" :disabled="creating">
              {{ creating ? 'Creating…' : 'Create key' }}
            </button>
          </form>
        </section>
        <div class="toolbar">
          <button type="button" class="neutral" :disabled="loading" @click="loadConfig">
            {{ loading ? 'Loading…' : 'Reload' }}
          </button>
        </div>
        <div class="table-wrap">
          <table class="cfg-table">
            <thead>
              <tr>
                <th scope="col">Key</th>
                <th scope="col">Value</th>
                <th scope="col" class="narrow">Save</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="v in rows" :key="v.key">
                <td class="mono">{{ v.key }}</td>
                <td>
                  <input
                    v-model="edits[v.key]"
                    class="val-input mono"
                    :type="v.redacted ? 'password' : 'text'"
                    :placeholder="v.redacted ? 'new value (hidden)' : ''"
                    :aria-label="'Value for ' + v.key"
                  />
                  <span v-if="v.redacted" class="hint">redacted</span>
                </td>
                <td class="narrow">
                  <button
                    type="button"
                    class="small good"
                    :disabled="savingKey === v.key"
                    @click="saveRow(v.key)"
                  >
                    {{ savingKey === v.key ? '…' : 'Save' }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </Section>
    </main>
  </div>
</template>

<style scoped>
.shell {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}
.config-main {
  padding: 1rem 1.5rem 2rem;
}
.crumb {
  font-size: 0.9rem;
  margin-bottom: 0.75rem;
}
.crumb a {
  color: #2563eb;
  text-decoration: none;
}
.crumb a:hover {
  text-decoration: underline;
}
.sep {
  margin: 0 0.35rem;
  color: #64748b;
}
.lede {
  color: #475569;
  font-size: 0.95rem;
  line-height: 1.5;
  margin: 0 0 1rem;
}
.lede code {
  font-size: 0.85em;
  background: #f1f5f9;
  padding: 0.1em 0.35em;
  border-radius: 4px;
}
.err {
  background: #fef2f2;
  color: #b91c1c;
  padding: 0.65rem 0.85rem;
  border-radius: 6px;
  margin: 0 0 1rem;
}
.toolbar {
  margin-bottom: 0.75rem;
}
.create-panel {
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 0.75rem;
  margin-bottom: 1rem;
}
.create-panel h2 {
  margin: 0 0 0.5rem;
  font-size: 1rem;
}
.create-form {
  display: flex;
  flex-wrap: wrap;
  gap: 0.65rem;
  align-items: end;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
}
.field span {
  font-size: 0.8rem;
  color: #475569;
}
.small {
  padding: 0.3rem 0.55rem;
  font-size: 0.8rem;
}
.table-wrap {
  overflow-x: auto;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}
.cfg-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.9rem;
}
.cfg-table th,
.cfg-table td {
  padding: 0.5rem 0.65rem;
  text-align: left;
  border-bottom: 1px solid #e2e8f0;
}
.cfg-table th {
  background: #f8fafc;
  font-weight: 600;
}
.cfg-table tr:last-child td {
  border-bottom: none;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.85rem;
}
.val-input {
  width: 100%;
  max-width: 28rem;
  box-sizing: border-box;
  padding: 0.35rem 0.5rem;
  border: 1px solid #cbd5e1;
  border-radius: 4px;
}
.hint {
  margin-left: 0.5rem;
  font-size: 0.75rem;
  color: #64748b;
}
.narrow {
  width: 5rem;
  white-space: nowrap;
}
</style>
