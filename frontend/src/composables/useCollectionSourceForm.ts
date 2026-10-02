import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { create } from '@bufbuild/protobuf'
import { ConnectError } from '@connectrpc/connect'
import { getControllerClient } from '../api/controllerClient'
import { describeCronLine, isValidCronLine } from '../utils/cronHuman'
import { notifyRunEnqueued, notifySuccess } from '../utils/notify'
import { pollAfterCollectionRun } from '../utils/pollAfterRun'
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

export function useCollectionSourceForm() {
  const route = useRoute()
  const router = useRouter()

  const loading = ref(true)
  const loadErr = ref<string | null>(null)
  const formErr = ref<string | null>(null)
  const saving = ref(false)
  const runNowPending = ref(false)

  const collectorSourceSchemas = ref<CollectorSourceSchema[]>([])
  const collectorHeartbeatTypes = ref<string[]>([])
  const editingSourceLastRun = ref<bigint>(0n)

  const editId = ref('')
  const formCollectorType = ref('')
  const formSourceSpec = ref('')
  const formCronLine = ref('0 0 * * *')
  const formEnabled = ref(true)
  const formSourceSpecAdvanced = ref(false)
  const formSchemaBuilder = ref<BuilderState | null>(null)

  const collectorTypeError = ref('')
  const sourcePatternError = ref('')
  const schemaArgErrors = ref<Record<string, string>>({})
  const sourceSpecError = ref('')
  const cronLineError = ref('')
  const pendingFocusTab = ref<string | null>(null)

  const isEditRoute = computed(() => route.name === 'source-edit')
  const oneOffMode = computed(() => route.query.oneOff === '1')
  const duplicateId = computed(() => {
    const raw = route.query.duplicate
    return typeof raw === 'string' ? raw.trim() : ''
  })

  const pageTitle = computed(() => {
    if (oneOffMode.value) {
      return 'One-off collection (not saved)'
    }
    if (isEditRoute.value) {
      return 'Edit collection source'
    }
    if (duplicateId.value) {
      return 'Duplicate collection source'
    }
    return 'Add collection source'
  })

  const formTabs = computed(() => {
    const tabs = [{ id: 'source', label: 'Source' }]
    if (!oneOffMode.value) {
      tabs.push({ id: 'schedule', label: 'Schedule' })
    }
    return tabs
  })

  const formCronSummary = computed(() => describeCronLine(formCronLine.value))

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

  const schemaPatternOptions = computed(() =>
    (activeParsedSchema.value?.primary_patterns ?? []).map((p) => ({
      value: p.id,
      label: p.example ? `${p.label} (${p.example})` : p.label,
    })),
  )

  const schemaModifierOptions = computed(() =>
    (activeParsedSchema.value?.modifiers ?? []).map((m) => ({
      value: m.id,
      label: `${m.label} +${m.syntax_suffix}`,
    })),
  )

  const selectedSchemaModifierIds = computed({
    get(): string[] {
      const b = formSchemaBuilder.value
      const mods = activeParsedSchema.value?.modifiers ?? []
      if (!b) {
        return []
      }
      return mods.filter((m) => b.modifiers[m.id]).map((m) => m.id)
    },
    set(ids: string[]) {
      const b = formSchemaBuilder.value
      const mods = activeParsedSchema.value?.modifiers ?? []
      if (!b) {
        return
      }
      for (const m of mods) {
        b.modifiers[m.id] = ids.includes(m.id)
      }
    },
  })

  const formCollectorTypeOptions = computed(() => {
    const set = new Set(collectorHeartbeatTypes.value)
    const cur = formCollectorType.value.trim()
    if (cur) {
      set.add(cur)
    }
    return Array.from(set).sort()
  })

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
    editingSourceLastRun.value = 0n
  }

  function populateFromSource(s: CollectionSource, asEdit: boolean) {
    editId.value = asEdit ? s.id : ''
    formCollectorType.value = s.collectorType
    formSourceSpec.value = s.sourceSpec
    formCronLine.value = s.cronLine ?? ''
    formEnabled.value = s.enabled
    if (asEdit) {
      editingSourceLastRun.value = s.lastRunUnixMs ?? 0n
    }
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

  function onSchemaPatternChange(value: string | number | boolean | null) {
    if (value === null || value === false) {
      return
    }
    onBuilderPatternChange(String(value))
  }

  function applyCronPreset(expr: string) {
    formCronLine.value = expr
  }

  function clearValidationErrors() {
    collectorTypeError.value = ''
    sourcePatternError.value = ''
    schemaArgErrors.value = {}
    sourceSpecError.value = ''
    cronLineError.value = ''
  }

  function validateSourceTab(): boolean {
    collectorTypeError.value = ''
    sourcePatternError.value = ''
    schemaArgErrors.value = {}
    sourceSpecError.value = ''

    if (!formCollectorType.value.trim()) {
      collectorTypeError.value = 'Select a collector type.'
    }

    if (showStructuredSourceSpec.value && formSchemaBuilder.value) {
      if (!formSchemaBuilder.value.patternId.trim()) {
        sourcePatternError.value = 'Select a source pattern.'
      }
      for (const arg of activePatternArgs.value) {
        if (!formSchemaBuilder.value.args[arg.id]?.trim()) {
          schemaArgErrors.value[arg.id] = `${arg.label} is required.`
        }
      }
    }

    if (!formSourceSpec.value.trim()) {
      sourceSpecError.value = 'Source spec is required.'
    }

    return (
      !collectorTypeError.value &&
      !sourcePatternError.value &&
      !sourceSpecError.value &&
      Object.keys(schemaArgErrors.value).length === 0
    )
  }

  function validateScheduleTab(): boolean {
    cronLineError.value = ''
    if (oneOffMode.value) {
      return true
    }
    if (!isValidCronLine(formCronLine.value)) {
      cronLineError.value =
        'Use five fields — minute, hour, day of month, month, weekday — or leave empty.'
      return false
    }
    return true
  }

  /** Returns the first tab id that failed validation, or null when all tabs pass. */
  function validateAllTabs(): string | null {
    clearValidationErrors()
    if (!validateSourceTab()) {
      return 'source'
    }
    if (!validateScheduleTab()) {
      return 'schedule'
    }
    return null
  }

  function blockUntilTabsValid(): boolean {
    const invalidTab = validateAllTabs()
    if (invalidTab) {
      formErr.value = 'Complete all required fields on each tab before continuing.'
      pendingFocusTab.value = invalidTab
      return false
    }
    formErr.value = null
    return true
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

  async function loadCollectorSourceSchemas() {
    const res = await getControllerClient().listCollectorSourceSchemas(
      create(ListCollectorSourceSchemasRequestSchema, { collectorType: '' }),
    )
    collectorSourceSchemas.value = [...res.schemas]
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

  async function findSourceById(id: string): Promise<CollectionSource | null> {
    const res = await getControllerClient().listCollectionSources(
      create(ListCollectionSourcesRequestSchema, { collectorType: '' }),
    )
    return res.sources.find((s) => s.id === id) ?? null
  }

  async function initializeForm() {
    loading.value = true
    loadErr.value = null
    formErr.value = null
    resetForm()

    try {
      await loadCollectorTypesFromHeartbeats()
      await loadCollectorSourceSchemas()

      if (isEditRoute.value) {
        const id = String(route.params.id ?? '').trim()
        if (!id) {
          loadErr.value = 'Missing collection source id.'
          return
        }
        const source = await findSourceById(id)
        if (!source) {
          loadErr.value = `Collection source "${id}" was not found.`
          return
        }
        populateFromSource(source, true)
      } else if (duplicateId.value) {
        const source = await findSourceById(duplicateId.value)
        if (!source) {
          loadErr.value = `Collection source "${duplicateId.value}" was not found.`
          return
        }
        populateFromSource(source, false)
      } else if (oneOffMode.value) {
        formSourceSpecAdvanced.value = true
        formSchemaBuilder.value = null
        formCronLine.value = ''
      } else {
        formCollectorType.value = defaultCollectorType()
      }

      syncSchemaBuilderFromForm()
    } catch (e) {
      loadErr.value = e instanceof ConnectError ? e.message : String(e)
    } finally {
      loading.value = false
    }
  }

  function cancel() {
    void router.push({ name: 'sources' })
  }

  function onFormSubmit() {
    if (!blockUntilTabsValid()) {
      return
    }
    if (oneOffMode.value) {
      void runOneOff()
      return
    }
    void persistSource(false)
  }

  async function runOneOff() {
    saving.value = true
    runNowPending.value = true
    try {
      const ephemeral = create(CollectionSourceSchema, {
        id: '',
        collectorType: formCollectorType.value.trim(),
        sourceSpec: formSourceSpec.value.trim(),
        cronLine: formCronLine.value.trim(),
        enabled: formEnabled.value,
      })
      await getControllerClient().enqueueCollectionRequest(
        create(EnqueueCollectionRequestRequestSchema, {
          target: { case: 'ephemeralCollection', value: ephemeral },
        }),
      )
      notifyRunEnqueued('One-off collection enqueued.')
      await router.push({ name: 'sources' })
    } catch (e) {
      formErr.value = e instanceof ConnectError ? e.message : String(e)
    } finally {
      saving.value = false
      runNowPending.value = false
    }
  }

  async function persistSource(runAfterSave: boolean) {
    const spec = formSourceSpec.value.trim()
    const ct = formCollectorType.value.trim()
    const sourceIdForRun = editId.value.trim()
    if (runAfterSave && !sourceIdForRun) {
      formErr.value = 'Run now is only available when editing an existing source.'
      return
    }
    saving.value = true
    try {
      const source = create(CollectionSourceSchema, {
        id: editId.value,
        collectorType: ct,
        sourceSpec: spec,
        cronLine: formCronLine.value.trim(),
        enabled: formEnabled.value,
      })
      await getControllerClient().upsertCollectionSource(
        create(UpsertCollectionSourceRequestSchema, { source }),
      )
      if (runAfterSave) {
        runNowPending.value = true
        try {
          const beforeRun = editingSourceLastRun.value
          await getControllerClient().enqueueCollectionRequest(
            create(EnqueueCollectionRequestRequestSchema, {
              target: { case: 'collectionSourceId', value: sourceIdForRun },
            }),
          )
          notifyRunEnqueued('Source saved and collection run enqueued.')
          void pollAfterCollectionRun(
            beforeRun,
            async () => {
              const refreshed = await findSourceById(sourceIdForRun)
              if (refreshed) {
                editingSourceLastRun.value = refreshed.lastRunUnixMs ?? 0n
              }
            },
            () => editingSourceLastRun.value,
          )
        } finally {
          runNowPending.value = false
        }
      } else {
        notifySuccess(editId.value ? 'Collection source updated.' : 'Collection source created.')
      }
      await router.push({ name: 'sources' })
    } catch (e) {
      formErr.value = e instanceof ConnectError ? e.message : String(e)
    } finally {
      saving.value = false
    }
  }

  async function saveSourceAndRunNow() {
    if (!blockUntilTabsValid()) {
      return
    }
    await persistSource(true)
  }

  return {
    loading,
    loadErr,
    formErr,
    saving,
    runNowPending,
    oneOffMode,
    pageTitle,
    formTabs,
    formCollectorType,
    formSourceSpec,
    formCronLine,
    formEnabled,
    formSourceSpecAdvanced,
    formSchemaBuilder,
    formCronSummary,
    showStructuredSourceSpec,
    canUseStructuredForm,
    activeParsedSchema,
    activeSchemaCronHint,
    activePatternArgs,
    schemaPatternOptions,
    schemaModifierOptions,
    selectedSchemaModifierIds,
    formCollectorTypeOptions,
    editId,
    collectorTypeError,
    sourcePatternError,
    schemaArgErrors,
    sourceSpecError,
    cronLineError,
    pendingFocusTab,
    initializeForm,
    onFormCollectorTypeChange,
    toggleSourceSpecRaw,
    onSchemaPatternChange,
    applyCronPreset,
    cancel,
    onFormSubmit,
    saveSourceAndRunNow,
  }
}
