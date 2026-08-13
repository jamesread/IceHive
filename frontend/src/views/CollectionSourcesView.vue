<script setup lang="ts">
import { ref, computed, watch, onMounted, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { create } from '@bufbuild/protobuf'
import { ConnectError } from '@connectrpc/connect'
import AppHeader from '../components/AppHeader.vue'
import AppFooter from '../components/AppFooter.vue'
import QuickSearch from 'picocrank/vue/components/QuickSearch.vue'
import Section from 'picocrank/vue/components/Section.vue'
import Table from 'picocrank/vue/components/Table.vue'
import { getControllerClient } from '../api/controllerClient'
import { describeCronLine } from '../utils/cronHuman'
import {
  builderStateForPattern,
  composeSpec,
  defaultBuilderState,
  parseSourceSchemaDoc,
  parseSpecIntoBuilder,
  type BuilderState,
  type SourceSchemaDoc,
} from '../utils/sourceSchemaForm'
import type { CollectionSource, CollectorSourceSchema } from '../gen/icehive/v1/controller_pb'
import {
  CollectionSourceSchema,
  EnqueueCollectionRequestRequestSchema,
  ListCollectionSourcesRequestSchema,
  ListCollectorSourceSchemasRequestSchema,
  ListServicesRequestSchema,
  UpsertCollectionSourceRequestSchema,
} from '../gen/icehive/v1/controller_pb'

const route = useRoute()
const router = useRouter()

const sources = ref<CollectionSource[]>([])
/** Latest SourceSchema rows from the controller (populated from collector AMQP at startup). */
const collectorSourceSchemas = ref<CollectorSourceSchema[]>([])
const filterCollectorType = ref('')
const loading = ref(false)
const err = ref<string | null>(null)
const saving = ref(false)
/** Collection source id currently sending EnqueueCollectionRequest, or empty when idle. */
const runNowPendingId = ref('')

const formDialogRef = ref<HTMLDialogElement | null>(null)
/** When true, the dialog only enqueues an ephemeral CollectionSource (nothing persisted). */
const oneOffMode = ref(false)
const formDialogTitle = computed(() => {
  if (oneOffMode.value) {
    return 'One-off collection (not saved)'
  }
  return editId.value ? 'Edit collection source' : 'Add collection source'
})

/** Service names from heartbeats that look like collectors (`collector-*`). */
const collectorHeartbeatTypes = ref<string[]>([])

const editId = ref('')
const formCollectorType = ref('')
const formSourceSpec = ref('')
/** Five-field cron: minute hour day-of-month month day-of-week */
const formCronLine = ref('0 0 * * *')
const formEnabled = ref(true)
/** When true, user edits `source_spec` as free text instead of pattern + args + modifiers. */
const formSourceSpecAdvanced = ref(false)
/** Parsed schema-driven form state; null when no structured schema for the selected collector. */
const formSchemaBuilder = ref<BuilderState | null>(null)

const formCronSummary = computed(() => describeCronLine(formCronLine.value))

const sourceTableHeaders = [
  { key: 'collectorType', label: 'Collector', sortable: true },
  { key: 'sourceSpec', label: 'Spec', sortable: true },
  { key: 'cronLine', label: 'Schedule', sortable: true },
  { key: 'enabled', label: 'On', sortable: true },
  { key: 'lastSuccessUnixMs', label: 'Last success', sortable: true },
  { key: 'nextDueUnixMs', label: 'Next due', sortable: true },
  { key: 'run', label: 'Run' },
]

const activeParsedSchema = computed((): SourceSchemaDoc | null => {
  const ct = formCollectorType.value.trim()
  const row = collectorSourceSchemas.value.find((s) => (s.collectorType ?? '').trim() === ct)
  const raw = row?.bodyJson?.trim()
  if (!raw) {
    return null
  }
  return parseSourceSchemaDoc(raw)
})

const showStructuredSourceSpec = computed(
  () =>
    !formSourceSpecAdvanced.value &&
    formSchemaBuilder.value !== null &&
    (activeParsedSchema.value?.primary_patterns?.length ?? 0) > 0,
)

const canUseStructuredForm = computed(() => (activeParsedSchema.value?.primary_patterns?.length ?? 0) > 0)

const activeSchemaCronHint = computed(() => activeParsedSchema.value?.cron)

const activePatternArgs = computed(() => {
  const doc = activeParsedSchema.value
  const b = formSchemaBuilder.value
  if (!doc || !b) {
    return []
  }
  return doc.primary_patterns.find((x) => x.id === b.patternId)?.args ?? []
})

/** Options for the form select: heartbeats plus current value when editing a type not in the list. */
const formCollectorTypeOptions = computed(() => {
  const set = new Set(collectorHeartbeatTypes.value)
  const cur = formCollectorType.value.trim()
  if (cur) {
    set.add(cur)
  }
  return Array.from(set).sort()
})

/** Options for the filter dropdown (heartbeats only). */
const filterCollectorTypeOptions = computed(() => [...collectorHeartbeatTypes.value])

function defaultCollectorType(): string {
  const opts = collectorHeartbeatTypes.value
  return opts.length > 0 ? opts[0] : ''
}

function resetForm() {
  editId.value = ''
  formCollectorType.value = defaultCollectorType()
  formSourceSpec.value = ''
  formCronLine.value = '0 0 * * *'
  formEnabled.value = true
  formSourceSpecAdvanced.value = false
  formSchemaBuilder.value = null
}

function onFormCollectorTypeChange() {
  if (oneOffMode.value) {
    return
  }
  syncSchemaBuilderFromForm()
}

function syncSchemaBuilderFromForm() {
  if (oneOffMode.value) {
    return
  }
  const doc = activeParsedSchema.value
  if (!doc || doc.primary_patterns.length === 0) {
    formSchemaBuilder.value = null
    return
  }
  const parsed = parseSpecIntoBuilder(doc, formSourceSpec.value)
  if (parsed) {
    formSchemaBuilder.value = parsed
    formSourceSpecAdvanced.value = false
    return
  }
  if (formSourceSpec.value.trim()) {
    formSourceSpecAdvanced.value = true
    formSchemaBuilder.value = null
    return
  }
  formSchemaBuilder.value = defaultBuilderState(doc)
  formSourceSpecAdvanced.value = false
}

function toggleSourceSpecRaw() {
  formSourceSpecAdvanced.value = !formSourceSpecAdvanced.value
  if (!formSourceSpecAdvanced.value) {
    syncSchemaBuilderFromForm()
  }
}

function onBuilderPatternChange(id: string) {
  const doc = activeParsedSchema.value
  if (!doc) {
    return
  }
  formSchemaBuilder.value = builderStateForPattern(doc, id, formSchemaBuilder.value)
}

watch(
  [activeParsedSchema, formSchemaBuilder, formSourceSpecAdvanced],
  () => {
    if (formSourceSpecAdvanced.value) {
      return
    }
    const doc = activeParsedSchema.value
    const b = formSchemaBuilder.value
    if (!doc || !b || doc.primary_patterns.length === 0) {
      return
    }
    formSourceSpec.value = composeSpec(doc, b)
  },
  { deep: true },
)

function openAddSource() {
  err.value = null
  oneOffMode.value = false
  resetForm()
  void nextTick(() => {
    syncSchemaBuilderFromForm()
    formDialogRef.value?.showModal()
  })
}

function openOneOffCollect() {
  err.value = null
  oneOffMode.value = true
  resetForm()
  formSourceSpecAdvanced.value = true
  formSchemaBuilder.value = null
  formCronLine.value = ''
  void nextTick(() => {
    formDialogRef.value?.showModal()
  })
}

function startEdit(s: CollectionSource) {
  err.value = null
  oneOffMode.value = false
  editId.value = s.id
  formCollectorType.value = s.collectorType
  formSourceSpec.value = s.sourceSpec
  formCronLine.value = s.cronLine ?? ''
  formEnabled.value = s.enabled
  void nextTick(() => {
    formDialogRef.value?.showModal()
    syncSchemaBuilderFromForm()
  })
}

function cancelFormDialog() {
  formDialogRef.value?.close()
}

function onFormDialogClose() {
  oneOffMode.value = false
  resetForm()
}

function onFormDialogBackdropClick(e: MouseEvent) {
  if (e.target === e.currentTarget) {
    cancelFormDialog()
  }
}

function fmtMs(ms: bigint | undefined): string {
  if (ms === undefined || ms === 0n) return '—'
  const n = Number(ms)
  if (!Number.isFinite(n) || n <= 0) return '—'
  return new Date(n).toLocaleString()
}

function applyCronPreset(expr: string) {
  formCronLine.value = expr
}

function hasLastError(s: CollectionSource): boolean {
  return (s.lastError ?? '').trim().length > 0
}

async function loadCollectorSourceSchemas() {
  const res = await getControllerClient().listCollectorSourceSchemas(
    create(ListCollectorSourceSchemasRequestSchema, { collectorType: '' }),
  )
  collectorSourceSchemas.value = [...res.schemas]
}

async function loadCollectorTypesFromHeartbeats() {
  try {
    const res = await getControllerClient().listServices(create(ListServicesRequestSchema, {}))
    const names = new Set<string>()
    for (const s of res.services) {
      const name = (s.serviceName ?? '').trim()
      if (name.startsWith('collector-')) {
        names.add(name)
      }
    }
    collectorHeartbeatTypes.value = Array.from(names).sort()
    if (!editId.value && !formCollectorType.value.trim() && collectorHeartbeatTypes.value.length > 0) {
      formCollectorType.value = collectorHeartbeatTypes.value[0]
    }
  } catch (e) {
    collectorHeartbeatTypes.value = []
    throw e
  }
}

async function loadSources(withLoading = true) {
  if (withLoading) {
    err.value = null
    loading.value = true
  }
  try {
    const res = await getControllerClient().listCollectionSources(
      create(ListCollectionSourcesRequestSchema, {
        collectorType: filterCollectorType.value.trim(),
      }),
    )
    sources.value = [...res.sources]
  } catch (e) {
    err.value = e instanceof ConnectError ? e.message : String(e)
  } finally {
    if (withLoading) {
      loading.value = false
    }
  }
}

async function reloadAll() {
  err.value = null
  loading.value = true
  try {
    await loadCollectorTypesFromHeartbeats()
    await loadCollectorSourceSchemas()
    await loadSources(false)
  } catch (e) {
    err.value = e instanceof ConnectError ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

/** Persist the dialog form; when `runAfterSave`, enqueue an immediate collection run (edit mode only). */
function onFormSubmit() {
  if (oneOffMode.value) {
    void runOneOffFromDialog()
    return
  }
  void saveSource()
}

async function runOneOffFromDialog() {
  err.value = null
  const spec = formSourceSpec.value.trim()
  const ct = formCollectorType.value.trim()
  if (!ct) {
    err.value = 'Collector type is required.'
    return
  }
  if (!spec) {
    err.value = 'Source spec is required.'
    return
  }
  const cron = formCronLine.value.trim()
  saving.value = true
  runNowPendingId.value = '__oneoff__'
  try {
    const ephemeral = create(CollectionSourceSchema, {
      id: '',
      collectorType: ct,
      sourceSpec: spec,
      cronLine: cron,
      enabled: formEnabled.value,
    })
    await getControllerClient().enqueueCollectionRequest(
      create(EnqueueCollectionRequestRequestSchema, {
        target: { case: 'ephemeralCollection', value: ephemeral },
      }),
    )
    formDialogRef.value?.close()
  } catch (e) {
    err.value = e instanceof ConnectError ? e.message : String(e)
  } finally {
    saving.value = false
    runNowPendingId.value = ''
  }
}

async function persistSource(runAfterSave: boolean) {
  err.value = null
  const spec = formSourceSpec.value.trim()
  const ct = formCollectorType.value.trim()
  if (!ct) {
    err.value = 'Collector type is required.'
    return
  }
  if (!spec) {
    err.value = 'Source spec is required.'
    return
  }
  const sourceIdForRun = editId.value.trim()
  if (runAfterSave && !sourceIdForRun) {
    err.value = 'Run now is only available when editing an existing source.'
    return
  }
  const cron = formCronLine.value.trim()
  saving.value = true
  try {
    const source = create(CollectionSourceSchema, {
      id: editId.value,
      collectorType: ct,
      sourceSpec: spec,
      cronLine: cron,
      enabled: formEnabled.value,
    })
    await getControllerClient().upsertCollectionSource(
      create(UpsertCollectionSourceRequestSchema, { source }),
    )
    if (runAfterSave) {
      runNowPendingId.value = sourceIdForRun
      try {
        await getControllerClient().enqueueCollectionRequest(
          create(EnqueueCollectionRequestRequestSchema, {
            target: { case: 'collectionSourceId', value: sourceIdForRun },
          }),
        )
      } finally {
        runNowPendingId.value = ''
      }
    }
    await loadSources()
    formDialogRef.value?.close()
  } catch (e) {
    err.value = e instanceof ConnectError ? e.message : String(e)
  } finally {
    saving.value = false
  }
}

async function saveSource() {
  await persistSource(false)
}

async function saveSourceAndRunNow() {
  await persistSource(true)
}

async function runCollectionNow(s: CollectionSource) {
  err.value = null
  runNowPendingId.value = s.id
  try {
    await getControllerClient().enqueueCollectionRequest(
      create(EnqueueCollectionRequestRequestSchema, {
        target: { case: 'collectionSourceId', value: s.id },
      }),
    )
    await loadSources(false)
  } catch (e) {
    err.value = e instanceof ConnectError ? e.message : String(e)
  } finally {
    runNowPendingId.value = ''
  }
}

async function openOneOffFromQueryIfNeeded() {
  if (route.query.oneOff !== '1') return
  openOneOffCollect()
  await router.replace({ name: 'sources' })
}

async function openEditFromQueryIfNeeded() {
  const raw = route.query.edit
  const id = typeof raw === 'string' ? raw.trim() : ''
  if (!id) return
  const s = sources.value.find((row) => row.id === id)
  await router.replace({ name: 'sources' })
  if (!s) {
    err.value = `Collection source "${id}" was not found for editing.`
    return
  }
  startEdit(s)
}

onMounted(async () => {
  await reloadAll()
  await openOneOffFromQueryIfNeeded()
  await openEditFromQueryIfNeeded()
})

watch(
  () => route.query.oneOff,
  (v) => {
    if (v === '1') {
      void openOneOffFromQueryIfNeeded()
    }
  },
)

watch(
  () => route.query.edit,
  (v) => {
    if (typeof v === 'string' && v.trim()) {
      void openEditFromQueryIfNeeded()
    }
  },
)
</script>

<template>
  <div class="shell">
    <AppHeader>
      <template #toolbar>
        <QuickSearch placeholder="Quick search..." />
      </template>
    </AppHeader>
    <main class="main">
      <Section title="Collection sources">
        <p class="lede">
          Define opaque <code>source_spec</code> strings per <code>collector_type</code> and an optional
          <strong>cron schedule</strong> (five fields: minute, hour, day of month, month, weekday). Leave cron empty for
          <strong>run-now only</strong> sources (no polling; use <strong>Run now</strong>).
          <strong>One-off run</strong> sends the same collection message without creating a row in the controller
          database. Each collector interprets its own specs (for example GitHub:
          <code>repo:jamesread/faridoon</code> for one repo, <code>org.repos:olivetin</code> for every repo under a user
          or org. Optional modifiers after the primary spec: <code>+dependabot</code> (alerts; skipped for archived
          repos), <code>+pr</code> (pull requests; skipped for archived repos).
        </p>
        <p v-if="err" class="err" role="alert">{{ err }}</p>

        <div class="toolbar">
          <label class="filter">
            <span>Filter by collector type</span>
            <select
              v-model="filterCollectorType"
              class="mono"
              @change="() => void loadSources()"
            >
              <option value="">All</option>
              <option v-for="t in filterCollectorTypeOptions" :key="t" :value="t">
                {{ t }}
              </option>
            </select>
          </label>
          <button type="button" class="neutral" :disabled="loading" @click="reloadAll">
            {{ loading ? 'Loading…' : 'Reload' }}
          </button>
          <button type="button" class="good" :disabled="loading" @click="openAddSource">Add source</button>
          <button type="button" class="good" :disabled="loading" @click="openOneOffCollect">
            One-off run…
          </button>
        </div>
        <p v-if="collectorHeartbeatTypes.length === 0" class="hint warn">
          No <code>collector-*</code> service heartbeats yet. Start a collector to populate the dropdowns.
        </p>

        <Table :headers="sourceTableHeaders" :data="sources">
          <template #cell-collectorType="{ row, value }">
            <span class="collector-cell">
              <span class="mono">{{ value }}</span>
              <span v-if="hasLastError(row)" class="annotation bad">
                <span class="annotation-key">error</span>
                <span class="annotation-val">see details</span>
              </span>
            </span>
          </template>
          <template #cell-sourceSpec="{ row, value }">
            <router-link class="mono spec-link" :to="{ name: 'collector-details', params: { id: row.id } }">
              {{ value }}
            </router-link>
          </template>
          <template #cell-cronLine="{ row, value }">
            <div class="schedule-cell">
              <div class="mono cron-line">{{ value }}</div>
              <div class="cron-desc">{{ describeCronLine(row.cronLine) }}</div>
            </div>
          </template>
          <template #cell-enabled="{ value }">
            {{ value ? 'yes' : 'no' }}
          </template>
          <template #cell-lastSuccessUnixMs="{ value }">
            <span class="mono">{{ fmtMs(value) }}</span>
          </template>
          <template #cell-nextDueUnixMs="{ value }">
            <span class="mono">{{ fmtMs(value) }}</span>
          </template>
          <template #cell-run="{ row }">
            <button
              type="button"
              class="small good"
              :disabled="runNowPendingId !== ''"
              title="Publish a CollectionRequest for this source (runs immediately, ignoring schedule)"
              @click="runCollectionNow(row)"
            >
              {{ runNowPendingId === row.id ? '…' : 'Run now' }}
            </button>
          </template>
        </Table>
        <p class="hint table-hint">
          Open a source via its spec link for full details (including id and last error). Use
          <strong>Add source</strong> here, or <strong>Edit</strong> from the details page. Empty cron means run-now
          only — use <strong>Run now</strong> to collect.
        </p>
      </Section>
    </main>

    <dialog
      ref="formDialogRef"
      class="form-dialog"
      aria-labelledby="form-dialog-title"
      @close="onFormDialogClose"
      @click="onFormDialogBackdropClick"
    >
      <div class="form-dialog-inner" @click.stop>
        <h2 id="form-dialog-title" class="form-dialog-title">{{ formDialogTitle }}</h2>
        <form class="form-grid" @submit.prevent="onFormSubmit">
          <label class="field">
            <span>Collector type</span>
            <select v-model="formCollectorType" class="mono" required @change="onFormCollectorTypeChange">
              <option v-if="formCollectorTypeOptions.length === 0" disabled value="">
                No collector-* heartbeats
              </option>
              <option v-for="t in formCollectorTypeOptions" :key="t" :value="t">
                {{ t }}
              </option>
            </select>
          </label>
          <fieldset
            v-if="!oneOffMode && showStructuredSourceSpec && activeParsedSchema && formSchemaBuilder"
            class="schema-builder wide"
          >
            <legend class="schema-legend">Source (from collector schema)</legend>
            <div class="schema-patterns">
              <label v-for="p in activeParsedSchema.primary_patterns" :key="p.id" class="schema-radio">
                <input
                  type="radio"
                  name="source-pattern"
                  :value="p.id"
                  :checked="formSchemaBuilder?.patternId === p.id"
                  @change="onBuilderPatternChange(p.id)"
                />
                <span>{{ p.label }}</span>
                <span v-if="p.example" class="schema-example mono">{{ p.example }}</span>
              </label>
            </div>
            <div class="schema-args">
              <label v-for="a in activePatternArgs" :key="a.id" class="field">
                <span>{{ a.label }}</span>
                <input v-model="formSchemaBuilder.args[a.id]" class="mono" type="text" autocomplete="off" />
              </label>
            </div>
            <div v-if="activeParsedSchema.modifiers?.length" class="schema-mods">
              <span class="mods-label">Include</span>
              <label v-for="m in activeParsedSchema.modifiers" :key="m.id" class="field check mod-check">
                <input v-model="formSchemaBuilder!.modifiers[m.id]" type="checkbox" />
                <span>{{ m.label }} <code class="mod-code">+{{ m.syntax_suffix }}</code></span>
              </label>
            </div>
            <button type="button" class="tiny neutral schema-raw-btn" @click="toggleSourceSpecRaw">Edit raw source spec</button>
          </fieldset>
          <p v-else-if="!oneOffMode && canUseStructuredForm && formSourceSpecAdvanced" class="wide schema-raw-banner">
            <button type="button" class="tiny neutral" @click="toggleSourceSpecRaw">Use structured form</button>
          </p>
          <label class="field wide">
            <span>Source spec</span>
            <input
              v-model="formSourceSpec"
              class="mono"
              type="text"
              required
              :readonly="showStructuredSourceSpec"
              :placeholder="
                showStructuredSourceSpec ? '' : 'e.g. org.repos:jamesread +dependabot +pr'
              "
              :title="
                showStructuredSourceSpec
                  ? 'Composed from pattern, arguments, and modifiers above'
                  : ''
              "
            />
          </label>
          <template v-if="!oneOffMode">
            <p v-if="activeSchemaCronHint?.description" class="field wide schema-cron-hint">
              {{ activeSchemaCronHint.description }}
            </p>
            <label class="field wide">
              <span>Cron schedule (optional)</span>
              <input
                v-model="formCronLine"
                class="mono cron-input"
                type="text"
                placeholder="empty = run now only, or e.g. 0 0 * * *"
                spellcheck="false"
                aria-describedby="cron-summary"
              />
            </label>
            <div class="cron-block wide">
              <p id="cron-summary" class="cron-summary" role="status">
                <strong>Summary:</strong> {{ formCronSummary }}
              </p>
              <div class="presets">
                <span class="presets-label">Presets:</span>
                <button type="button" class="tiny neutral" @click="applyCronPreset('0 0 * * *')">Daily midnight</button>
                <button type="button" class="tiny neutral" @click="applyCronPreset('0 * * * *')">Hourly</button>
                <button type="button" class="tiny neutral" @click="applyCronPreset('*/15 * * * *')">Every 15 min</button>
                <button type="button" class="tiny neutral" @click="applyCronPreset('0 0 * * 0')">Weekly (Sun 00:00)</button>
              </div>
            </div>
          </template>
          <label v-if="!oneOffMode" class="field check">
            <input v-model="formEnabled" type="checkbox" />
            <span>Enabled</span>
          </label>
          <p v-if="oneOffMode" class="form-dialog-hint wide">
            This publishes a <code>CollectionRequest</code> with an inline source (same shape as a saved source). Nothing
            is written to the controller database; run history is not updated for a source id. Cron is omitted (empty
            schedule: immediate run).
          </p>
          <p v-else class="form-dialog-hint">
            Use standard 5-field cron when set. Empty cron is allowed: collectors only run the source when you use
            <strong>Run now</strong>.
          </p>
          <div class="form-actions">
            <template v-if="oneOffMode">
              <button type="submit" class="good" :disabled="saving || runNowPendingId !== ''">
                {{ saving ? 'Working…' : 'Run once without saving' }}
              </button>
              <button type="button" class="neutral" :disabled="saving" @click="cancelFormDialog">Cancel</button>
            </template>
            <template v-else>
              <button type="submit" class="good" :disabled="saving">
                {{ saving ? 'Working…' : editId ? 'Update' : 'Create' }}
              </button>
              <button
                v-if="editId"
                type="button"
                class="good"
                :disabled="saving"
                title="Save changes and publish a collection request immediately"
                @click="saveSourceAndRunNow"
              >
                {{ saving ? 'Working…' : 'Update and run now' }}
              </button>
              <button type="button" class="neutral" :disabled="saving" @click="cancelFormDialog">Cancel</button>
            </template>
          </div>
        </form>
      </div>
    </dialog>

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
.collector-cell {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem;
}
.toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  align-items: flex-end;
  margin-bottom: 1rem;
}
.filter {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}
.filter span {
  font-size: 0.8rem;
  color: #475569;
}
.filter input,
.filter select {
  min-width: 14rem;
  padding: 0.35rem 0.5rem;
  border: 1px solid #cbd5e1;
  border-radius: 4px;
  background: #fff;
}
.field select {
  padding: 0.35rem 0.5rem;
  border: 1px solid #cbd5e1;
  border-radius: 4px;
  background: #fff;
  max-width: 100%;
}
.hint.warn {
  color: #b45309;
  background: #fffbeb;
  padding: 0.5rem 0.75rem;
  border-radius: 6px;
  border: 1px solid #fde68a;
}
.form-grid {
  display: grid;
  grid-template-columns: 1fr 2fr;
  gap: 0.65rem 1rem;
  align-items: end;
  margin-bottom: 0.5rem;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}
.field.wide {
  grid-column: 1 / -1;
}
.field.check {
  flex-direction: row;
  align-items: center;
  gap: 0.5rem;
}
.field span {
  font-size: 0.8rem;
  color: #475569;
}
.field input[type='text'],
.field input[type='number'] {
  padding: 0.35rem 0.5rem;
  border: 1px solid #cbd5e1;
  border-radius: 4px;
}
.cron-input {
  font-size: 0.9rem;
}
.cron-block.wide {
  grid-column: 1 / -1;
}
.cron-summary {
  margin: 0 0 0.5rem;
  font-size: 0.9rem;
  color: #334155;
  line-height: 1.45;
}
.presets {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem;
}
.presets-label {
  font-size: 0.8rem;
  color: #64748b;
  margin-right: 0.25rem;
}
.tiny {
  padding: 0.2rem 0.45rem;
  font-size: 0.75rem;
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
.form-actions {
  grid-column: 1 / -1;
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: center;
}
.hint {
  font-size: 0.8rem;
  color: #64748b;
  margin: 0 0 1rem;
}
.small {
  padding: 0.25rem 0.45rem;
  font-size: 0.8rem;
  margin-right: 0.25rem;
}
.table-hint {
  margin-top: 0.75rem;
}
.form-dialog {
  padding: 0;
  border: none;
  border-radius: 10px;
  max-width: min(48rem, calc(100vw - 2rem));
  width: 100%;
  box-shadow: 0 25px 50px -12px rgb(0 0 0 / 0.25);
}
.form-dialog::backdrop {
  background: rgb(15 23 42 / 0.45);
}
.form-dialog-inner {
  padding: 1.25rem 1.35rem 1.35rem;
}
.form-dialog-title {
  margin: 0 0 1rem;
  font-size: 1.15rem;
  font-weight: 600;
  color: #0f172a;
}
.form-dialog-hint {
  grid-column: 1 / -1;
  margin: 0;
  font-size: 0.8rem;
  color: #64748b;
  line-height: 1.45;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
.spec-link {
  color: #2563eb;
  text-decoration: none;
}
.spec-link:hover {
  text-decoration: underline;
}
.small {
  font-size: 0.75rem;
  max-width: 8rem;
  overflow: hidden;
  text-overflow: ellipsis;
}
.schema-builder {
  grid-column: 1 / -1;
  margin: 0;
  padding: 0.65rem 0.85rem;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #f8fafc;
}
.schema-legend {
  font-size: 0.82rem;
  font-weight: 600;
  color: #334155;
  padding: 0 0.25rem;
}
.schema-patterns {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  margin-bottom: 0.65rem;
}
.schema-radio {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem 0.5rem;
  font-size: 0.88rem;
  cursor: pointer;
}
.schema-example {
  font-size: 0.78rem;
  color: #64748b;
}
.schema-args {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem 1rem;
  margin-bottom: 0.5rem;
}
.schema-mods {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem 0.75rem;
  margin-bottom: 0.5rem;
}
.mods-label {
  font-size: 0.8rem;
  color: #64748b;
  margin-right: 0.25rem;
}
.mod-check {
  margin: 0;
}
.mod-code {
  font-size: 0.78em;
  background: #f1f5f9;
  padding: 0.05em 0.25em;
  border-radius: 3px;
}
.schema-raw-btn {
  margin-top: 0.25rem;
}
.schema-raw-banner {
  grid-column: 1 / -1;
  margin: 0;
}
.schema-cron-hint {
  font-size: 0.8rem;
  color: #64748b;
  line-height: 1.4;
  margin: 0;
}
</style>
