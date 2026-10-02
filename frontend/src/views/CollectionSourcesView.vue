<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { create } from '@bufbuild/protobuf'
import { ConnectError } from '@connectrpc/connect'
import { HugeiconsIcon } from '@hugeicons/vue'
import {
  Add01Icon,
  ArrowReloadHorizontalIcon,
  DatabaseIcon,
  FlashIcon,
} from '@hugeicons/core-free-icons'
import AppHeader from '../components/AppHeader.vue'
import AppFooter from '../components/AppFooter.vue'
import QuickSearch from 'picocrank/vue/components/QuickSearch.vue'
import Section from 'picocrank/vue/components/Section.vue'
import Table from 'picocrank/vue/components/Table.vue'
import { getControllerClient } from '../api/controllerClient'
import { describeCronLine } from '../utils/cronHuman'
import { notifyRunEnqueued } from '../utils/notify'
import { pollAfterCollectionRun } from '../utils/pollAfterRun'
import type { CollectionSource } from '../gen/icehive/v1/controller_pb'
import {
  EnqueueCollectionRequestRequestSchema,
  ListCollectionSourcesRequestSchema,
  ListServicesRequestSchema,
} from '../gen/icehive/v1/controller_pb'

const router = useRouter()

const sources = ref<CollectionSource[]>([])
const collectorHeartbeatTypes = ref<string[]>([])
const filterProblems = ref<'all' | 'stale' | 'error'>('all')
const loading = ref(false)
const listErr = ref<string | null>(null)
const runNowPendingId = ref('')

const tableHeaders = computed(() => [
  { key: 'collectorType', label: 'Collector', sortable: true, width: '10rem' },
  { key: 'sourceSpec', label: 'Spec', sortable: true },
  { key: 'cronLine', label: 'Schedule', sortable: true, width: '12rem' },
  { key: 'enabled', label: 'On', sortable: true, width: '5rem' },
  { key: 'lastSuccessUnixMs', label: 'Last success', sortable: true, width: '11rem' },
  { key: 'pipelineHealth', label: 'Pipeline', sortable: false, width: '12rem' },
  { key: 'nextDueUnixMs', label: 'Next due', sortable: true, width: '11rem' },
  { key: 'actions', label: 'Actions', sortable: false, width: '11rem' },
])

const displayedSources = computed(() => {
  let rows = sources.value
  if (filterProblems.value === 'stale') {
    rows = rows.filter((s) => s.enabled && s.isStale)
  } else if (filterProblems.value === 'error') {
    rows = rows.filter((s) => hasLastError(s))
  }
  return rows
})

const tableRows = computed(() =>
  displayedSources.value.map((row) => ({
    ...row,
    pipelineHealth: pipelineHealthLabel(row),
  })),
)

function fmtMs(ms: bigint | undefined): string {
  if (ms === undefined || ms === 0n) return '—'
  const n = Number(ms)
  if (!Number.isFinite(n) || n <= 0) return '—'
  return new Date(n).toLocaleString()
}

function hasLastError(s: CollectionSource): boolean {
  return (s.lastError ?? '').trim().length > 0
}

function truncateError(msg: string, max = 72): string {
  const t = msg.trim()
  if (t.length <= max) return t
  return `${t.slice(0, max - 1)}…`
}

function fmtAgeSeconds(sec: bigint | undefined): string {
  if (sec === undefined || sec === 0n) return '—'
  const n = Number(sec)
  if (!Number.isFinite(n) || n < 0) return '—'
  if (n < 60) return `${n}s ago`
  const min = Math.floor(n / 60)
  if (min < 60) return `${min}m ago`
  const hr = Math.floor(min / 60)
  if (hr < 48) return `${hr}h ago`
  const day = Math.floor(hr / 24)
  return `${day}d ago`
}

function pipelineHealthLabel(s: CollectionSource): string {
  if (!s.enabled) return 'disabled'
  if (s.isStale) return 'stale'
  return 'healthy'
}

async function loadCollectorTypesFromHeartbeats() {
  const res = await getControllerClient().listServices(create(ListServicesRequestSchema, {}))
  const names = new Set<string>()
  for (const s of res.services) {
    const name = (s.serviceName ?? '').trim()
    if (name.startsWith('collector-')) {
      names.add(name)
    }
  }
  collectorHeartbeatTypes.value = Array.from(names).sort()
}

async function loadSources(withLoading = true) {
  if (withLoading) {
    listErr.value = null
    loading.value = true
  }
  try {
    const res = await getControllerClient().listCollectionSources(
      create(ListCollectionSourcesRequestSchema, {
        collectorType: '',
      }),
    )
    sources.value = [...res.sources]
  } catch (e) {
    listErr.value = e instanceof ConnectError ? e.message : String(e)
  } finally {
    if (withLoading) {
      loading.value = false
    }
  }
}

async function reloadAll() {
  listErr.value = null
  loading.value = true
  try {
    await loadCollectorTypesFromHeartbeats()
    await loadSources(false)
  } catch (e) {
    listErr.value = e instanceof ConnectError ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

async function runCollectionNow(s: CollectionSource) {
  listErr.value = null
  runNowPendingId.value = s.id
  const beforeRun = s.lastRunUnixMs ?? 0n
  try {
    await getControllerClient().enqueueCollectionRequest(
      create(EnqueueCollectionRequestRequestSchema, {
        target: { case: 'collectionSourceId', value: s.id },
      }),
    )
    notifyRunEnqueued()
    void pollAfterCollectionRun(
      beforeRun,
      () => loadSources(false),
      () => sources.value.find((row) => row.id === s.id)?.lastRunUnixMs,
    )
  } catch (e) {
    listErr.value = e instanceof ConnectError ? e.message : String(e)
  } finally {
    runNowPendingId.value = ''
  }
}

function openSourceDetail({ row }: { row: CollectionSource }) {
  void router.push({ name: 'collector-details', params: { id: row.id } })
}

function openAddSource() {
  void router.push({ name: 'source-create' })
}

function openOneOffCollect() {
  void router.push({ name: 'source-create', query: { oneOff: '1' } })
}

function duplicateSource(s: CollectionSource) {
  void router.push({ name: 'source-create', query: { duplicate: s.id } })
}

onMounted(() => {
  void reloadAll()
})
</script>

<template>
  <div class="shell">
    <AppHeader>
      <template #toolbar>
        <QuickSearch placeholder="Quick search..." />
      </template>
    </AppHeader>
    <main class="main">
      <Section
        title="Collection sources"
        :icon="DatabaseIcon"
        subtitle="Define collector targets, opaque source specs, and cron schedules. Click a row for details; use Run now for immediate collection."
        classes="collection-sources-list"
        :padding="false"
      >
        <template #toolbar>
          <button type="button" class="neutral" title="Refresh" :disabled="loading" @click="reloadAll">
            <HugeiconsIcon :icon="ArrowReloadHorizontalIcon" width="1em" height="1em" aria-hidden="true" />
          </button>
          <label class="toolbar-filter">
            <span class="sr-only">Show</span>
            <select v-model="filterProblems" class="mono" title="Filter by pipeline status">
              <option value="all">All sources</option>
              <option value="stale">Stale pipeline only</option>
              <option value="error">Last error only</option>
            </select>
          </label>
          <button type="button" class="neutral" title="One-off run" :disabled="loading" @click="openOneOffCollect">
            <HugeiconsIcon :icon="FlashIcon" width="1em" height="1em" aria-hidden="true" />
          </button>
          <button type="button" class="good" title="Add source" :disabled="loading" @click="openAddSource">
            <HugeiconsIcon :icon="Add01Icon" width="1em" height="1em" aria-hidden="true" />
          </button>
        </template>

        <div v-if="listErr" class="inline-notification error list-banner-pad">{{ listErr }}</div>
        <div v-if="loading && !sources.length" class="list-banner-pad muted">Loading…</div>

        <template v-else>
          <p v-if="collectorHeartbeatTypes.length === 0" class="inline-notification note list-banner-pad">
            No <code>collector-*</code> service heartbeats yet. Start a collector to populate the form dropdowns.
          </p>
          <p v-if="!displayedSources.length && !sources.length" class="inline-notification note list-banner-pad">
            No collection sources yet.
          </p>
          <p v-else-if="!displayedSources.length" class="inline-notification note list-banner-pad">
            No sources match the current filters.
          </p>

          <Table
            v-else
            class="list-table-wrap"
            row-clickable
            :headers="tableHeaders"
            :data="tableRows"
            @row-click="openSourceDetail"
          >
            <template #cell-collectorType="{ value }">
              <span class="mono">{{ value }}</span>
            </template>
            <template #cell-sourceSpec="{ value }">
              <strong>{{ value }}</strong>
            </template>
            <template #cell-cronLine="{ row, value }">
              <div class="schedule-cell">
                <div class="mono cron-line">{{ value || '—' }}</div>
                <div v-if="value" class="cron-desc">{{ describeCronLine(row.cronLine) }}</div>
              </div>
            </template>
            <template #cell-enabled="{ value }">
              {{ value ? 'yes' : 'no' }}
            </template>
            <template #cell-lastSuccessUnixMs="{ value }">
              <span class="mono">{{ fmtMs(value) }}</span>
            </template>
            <template #cell-pipelineHealth="{ row }">
              <span
                class="annotation"
                :class="pipelineHealthLabel(row) === 'stale' ? 'bad' : pipelineHealthLabel(row) === 'healthy' ? 'good' : 'neutral'"
              >
                <span class="annotation-key">status</span>
                <span class="annotation-val">{{ pipelineHealthLabel(row) }}</span>
              </span>
              <div v-if="row.entityFreshnessAgeSeconds > 0n" class="freshness-hint mono">
                entities {{ fmtAgeSeconds(row.entityFreshnessAgeSeconds) }}
              </div>
              <div v-if="hasLastError(row)" class="last-error-hint">
                <span class="annotation bad">
                  <span class="annotation-key">error</span>
                  <span class="annotation-val" :title="row.lastError">{{ truncateError(row.lastError ?? '') }}</span>
                </span>
              </div>
            </template>
            <template #cell-nextDueUnixMs="{ value }">
              <span class="mono">{{ fmtMs(value) }}</span>
            </template>
            <template #cell-actions="{ row }">
              <div class="actions-cell">
                <button
                  type="button"
                  class="small good"
                  :disabled="runNowPendingId !== ''"
                  title="Publish a CollectionRequest for this source (runs immediately, ignoring schedule)"
                  @click="runCollectionNow(row)"
                >
                  {{ runNowPendingId === row.id ? '…' : 'Run now' }}
                </button>
                <button
                  type="button"
                  class="small neutral"
                  title="Copy this source into the add form"
                  @click="duplicateSource(row)"
                >
                  Duplicate
                </button>
              </div>
            </template>
          </Table>
        </template>
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
.main {
  flex: 1;
  padding: 1rem 1.5rem 2rem;
}
.list-banner-pad {
  padding-left: 1em;
  padding-right: 1em;
}
.list-table-wrap {
  margin-top: 0.5rem;
  margin-bottom: 1.5rem;
}
.toolbar-filter select {
  max-width: 11rem;
  padding: 0.35rem 0.5rem;
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}
.actions-cell {
  text-align: right;
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.35rem;
}
.schedule-cell {
  max-width: 14rem;
}
.cron-line {
  font-size: 0.82rem;
}
.cron-desc {
  font-size: 0.78rem;
  color: #64748b;
  margin-top: 0.2rem;
  line-height: 1.35;
}
.freshness-hint {
  margin-top: 0.25rem;
  font-size: 0.75rem;
  color: #64748b;
}
.last-error-hint {
  margin-top: 0.35rem;
  max-width: 18rem;
}
.last-error-hint .annotation-val {
  word-break: break-word;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
.small {
  padding: 0.25rem 0.45rem;
  font-size: 0.8rem;
}
</style>
