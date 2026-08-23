<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { create } from '@bufbuild/protobuf'
import { ConnectError } from '@connectrpc/connect'
import AppHeader from '../components/AppHeader.vue'
import AppFooter from '../components/AppFooter.vue'
import QuickSearch from 'picocrank/vue/components/QuickSearch.vue'
import Section from 'picocrank/vue/components/Section.vue'
import { getControllerClient } from '../api/controllerClient'
import { describeCronLine } from '../utils/cronHuman'
import { notifySuccess } from '../utils/notify'
import { pollAfterCollectionRun } from '../utils/pollAfterRun'
import type { CollectionSource } from '../gen/icehive/v1/controller_pb'
import {
  DeleteCollectionSourceRequestSchema,
  EnqueueCollectionRequestRequestSchema,
  ListCollectionSourcesRequestSchema,
} from '../gen/icehive/v1/controller_pb'

const route = useRoute()
const router = useRouter()

const source = ref<CollectionSource | null>(null)
const loading = ref(false)
const err = ref<string | null>(null)
const runNowPending = ref(false)

const sourceId = computed(() => String(route.params.id ?? '').trim())

const pageTitle = computed(() => {
  const spec = source.value?.sourceSpec?.trim()
  if (spec) return spec
  if (sourceId.value) return `Source ${sourceId.value.slice(0, 8)}`
  return 'Collection source'
})

const cronSummary = computed(() => describeCronLine(source.value?.cronLine ?? ''))

function fmtMs(ms: bigint | undefined): string {
  if (ms === undefined || ms === 0n) return '—'
  const n = Number(ms)
  if (!Number.isFinite(n) || n <= 0) return '—'
  return new Date(n).toLocaleString()
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

function pipelineStatusLabel(): string {
  const s = source.value
  if (!s) return '—'
  if (!s.enabled) return 'disabled'
  if (s.isStale) return 'stale'
  return 'healthy'
}

function syncDocumentTitle() {
  document.title = `${pageTitle.value} » IceHive`
}

async function loadSource() {
  const id = sourceId.value
  if (!id) {
    source.value = null
    err.value = 'Missing collection source id.'
    return
  }
  err.value = null
  loading.value = true
  try {
    const res = await getControllerClient().listCollectionSources(
      create(ListCollectionSourcesRequestSchema, { collectorType: '' }),
    )
    const found = res.sources.find((s) => s.id === id) ?? null
    source.value = found
    if (!found) {
      err.value = `Collection source "${id}" was not found.`
    }
  } catch (e) {
    source.value = null
    err.value = e instanceof ConnectError ? e.message : String(e)
  } finally {
    loading.value = false
    syncDocumentTitle()
  }
}

async function runCollectionNow() {
  const s = source.value
  if (!s) return
  err.value = null
  runNowPending.value = true
  const beforeRun = s.lastRunUnixMs ?? 0n
  try {
    await getControllerClient().enqueueCollectionRequest(
      create(EnqueueCollectionRequestRequestSchema, {
        target: { case: 'collectionSourceId', value: s.id },
      }),
    )
    notifySuccess('Collection run enqueued.')
    void pollAfterCollectionRun(
      beforeRun,
      () => loadSource(),
      () => source.value?.lastRunUnixMs,
    )
  } catch (e) {
    err.value = e instanceof ConnectError ? e.message : String(e)
  } finally {
    runNowPending.value = false
  }
}

async function removeSource() {
  const s = source.value
  if (!s) return
  if (!confirm(`Delete collection source ${s.id}?`)) return
  err.value = null
  try {
    await getControllerClient().deleteCollectionSource(
      create(DeleteCollectionSourceRequestSchema, { id: s.id }),
    )
    await router.push({ name: 'sources' })
  } catch (e) {
    err.value = e instanceof ConnectError ? e.message : String(e)
  }
}

function goEdit() {
  const s = source.value
  if (!s) return
  void router.push({ name: 'sources', query: { edit: s.id } })
}

function duplicateSource() {
  const s = source.value
  if (!s) return
  void router.push({
    name: 'sources',
    query: { duplicate: s.id },
  })
}

onMounted(() => {
  void loadSource()
})

watch(sourceId, () => {
  void loadSource()
})

watch(pageTitle, syncDocumentTitle)
</script>

<template>
  <div class="shell">
    <AppHeader>
      <template #toolbar>
        <QuickSearch placeholder="Quick search..." />
      </template>
    </AppHeader>
    <main class="main">
      <Section :title="pageTitle">
        <p class="lede">
          Full details for one collection source. Schedule, run history timestamps, and identity fields that are hidden
          from the sources list live here.
        </p>
        <p v-if="err" class="err" role="alert">{{ err }}</p>
        <p v-if="loading && !source" class="hint">Loading…</p>

        <template v-if="source">
          <div class="toolbar">
            <button type="button" class="neutral" :disabled="loading" @click="loadSource">
              {{ loading ? 'Loading…' : 'Reload' }}
            </button>
            <button
              type="button"
              class="good"
              :disabled="runNowPending"
              title="Publish a CollectionRequest for this source (runs immediately, ignoring schedule)"
              @click="runCollectionNow"
            >
              {{ runNowPending ? '…' : 'Run now' }}
            </button>
            <button type="button" class="neutral" @click="goEdit">Edit</button>
            <button type="button" class="neutral" @click="duplicateSource">Duplicate</button>
            <button type="button" class="bad" @click="removeSource">Delete</button>
            <button type="button" class="neutral" @click="router.push({ name: 'sources' })">Back to list</button>
          </div>

          <dl class="detail-grid">
            <div class="detail">
              <dt>ID</dt>
              <dd class="mono">{{ source.id }}</dd>
            </div>
            <div class="detail">
              <dt>Collector type</dt>
              <dd class="mono">{{ source.collectorType }}</dd>
            </div>
            <div class="detail wide">
              <dt>Source spec</dt>
              <dd class="mono">{{ source.sourceSpec }}</dd>
            </div>
            <div class="detail">
              <dt>Schedule (cron)</dt>
              <dd>
                <div class="mono">{{ source.cronLine || '—' }}</div>
                <div class="cron-desc">{{ cronSummary }}</div>
              </dd>
            </div>
            <div class="detail">
              <dt>Enabled</dt>
              <dd>{{ source.enabled ? 'yes' : 'no' }}</dd>
            </div>
            <div class="detail">
              <dt>Last run</dt>
              <dd class="mono">{{ fmtMs(source.lastRunUnixMs) }}</dd>
            </div>
            <div class="detail">
              <dt>Last success</dt>
              <dd class="mono">{{ fmtMs(source.lastSuccessUnixMs) }}</dd>
            </div>
            <div class="detail">
              <dt>Pipeline status</dt>
              <dd>
                <span
                  class="annotation"
                  :class="pipelineStatusLabel() === 'stale' ? 'bad' : pipelineStatusLabel() === 'healthy' ? 'good' : 'neutral'"
                >
                  <span class="annotation-key">status</span>
                  <span class="annotation-val">{{ pipelineStatusLabel() }}</span>
                </span>
                <div v-if="source.secondsSinceLastSuccess > 0n" class="cron-desc mono">
                  last success {{ fmtAgeSeconds(source.secondsSinceLastSuccess) }}
                </div>
                <div v-if="source.entityFreshnessAgeSeconds > 0n" class="cron-desc mono">
                  entity rows {{ fmtAgeSeconds(source.entityFreshnessAgeSeconds) }}
                </div>
              </dd>
            </div>
            <div class="detail">
              <dt>Next due</dt>
              <dd class="mono">{{ fmtMs(source.nextDueUnixMs) }}</dd>
            </div>
            <div class="detail">
              <dt>Created</dt>
              <dd class="mono">{{ fmtMs(source.createdUnixMs) }}</dd>
            </div>
            <div class="detail">
              <dt>Updated</dt>
              <dd class="mono">{{ fmtMs(source.updatedUnixMs) }}</dd>
            </div>
            <div class="detail wide">
              <dt>Last error</dt>
              <dd>
                <template v-if="(source.lastError ?? '').trim()">
                  <div class="annotation bad last-error-annotation">
                    <span class="annotation-key">error</span>
                    <span class="annotation-val">Last collection failed</span>
                  </div>
                  <pre class="err-body mono">{{ source.lastError }}</pre>
                </template>
                <span v-else>—</span>
              </dd>
            </div>
          </dl>
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
.lede {
  color: #475569;
  font-size: 0.95rem;
  line-height: 1.5;
  margin: 0 0 1rem;
}
.err {
  color: #b91c1c;
  margin: 0 0 0.75rem;
}
.hint {
  font-size: 0.8rem;
  color: #64748b;
  margin: 0 0 1rem;
}
.toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: end;
  margin-bottom: 1rem;
}
.detail-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(16rem, 1fr));
  gap: 0.85rem 1.25rem;
  margin: 0;
}
.detail {
  margin: 0;
  padding: 0.75rem 0.85rem;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #f8fafc;
}
.detail.wide {
  grid-column: 1 / -1;
}
.detail dt {
  margin: 0 0 0.35rem;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: #64748b;
}
.detail dd {
  margin: 0;
  color: #0f172a;
  line-height: 1.45;
  word-break: break-word;
}
.cron-desc {
  margin-top: 0.25rem;
  font-size: 0.8rem;
  color: #64748b;
}
.last-error-annotation {
  margin: 0 0 0.65rem;
}
.err-body {
  margin: 0;
  white-space: pre-wrap;
  font-size: 0.85rem;
  color: #b91c1c;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
</style>
