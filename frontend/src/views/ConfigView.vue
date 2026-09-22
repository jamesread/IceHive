<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { create } from '@bufbuild/protobuf'
import { ConnectError } from '@connectrpc/connect'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Add01Icon, ArrowReloadHorizontalIcon, Configuration01Icon } from '@hugeicons/core-free-icons'
import AppHeader from '../components/AppHeader.vue'
import AppFooter from '../components/AppFooter.vue'
import QuickSearch from 'picocrank/vue/components/QuickSearch.vue'
import Section from 'picocrank/vue/components/Section.vue'
import type { ConfigVar } from '../gen/icehive/v1/controller_pb'
import {
  ListConfigRequestSchema,
  SetConfigRequestSchema,
} from '../gen/icehive/v1/controller_pb'
import { getControllerClient } from '../api/controllerClient'
import { notifySuccess } from '../utils/notify'

const router = useRouter()
const rows = ref<ConfigVar[]>([])
const edits = ref<Record<string, string>>({})
const loading = ref(false)
const err = ref<string | null>(null)
const savingKey = ref<string | null>(null)

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
    notifySuccess(`Saved "${key}".`)
    await loadConfig()
  } catch (e) {
    err.value = e instanceof ConnectError ? e.message : String(e)
  } finally {
    savingKey.value = null
  }
}

function openCreate() {
  void router.push({ name: 'config-create' })
}

onMounted(() => {
  void loadConfig()
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
        title="Controller configuration"
        :icon="Configuration01Icon"
        subtitle="Values come from the controller via Connect RPC (ListConfig / SetConfig). Changes are written to controller metadata."
        :padding="false"
      >
        <template #toolbar>
          <button type="button" class="neutral" title="Reload" :disabled="loading" @click="loadConfig">
            <HugeiconsIcon :icon="ArrowReloadHorizontalIcon" width="1em" height="1em" aria-hidden="true" />
          </button>
          <button type="button" class="good" title="Create key" :disabled="loading" @click="openCreate">
            <HugeiconsIcon :icon="Add01Icon" width="1em" height="1em" aria-hidden="true" />
          </button>
        </template>

        <p v-if="err" class="err list-banner-pad" role="alert">{{ err }}</p>
        <div v-if="loading && !rows.length" class="list-banner-pad muted">Loading…</div>
        <div v-else class="table-wrap">
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
              <tr v-if="!rows.length">
                <td colspan="3">No configuration keys yet.</td>
              </tr>
            </tbody>
          </table>
        </div>
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
.list-banner-pad {
  padding-left: 1em;
  padding-right: 1em;
}
.muted {
  color: #64748b;
}
.err {
  background: #fef2f2;
  color: #b91c1c;
  padding: 0.65rem 0.85rem;
  border-radius: 6px;
  margin: 0 0 1rem;
}
.small {
  padding: 0.3rem 0.55rem;
  font-size: 0.8rem;
}
.table-wrap {
  overflow-x: auto;
  margin-top: 0.5rem;
  margin-bottom: 1rem;
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
