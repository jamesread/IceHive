<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { DatabaseIcon } from '@hugeicons/core-free-icons'
import AppHeader from '../components/AppHeader.vue'
import AppFooter from '../components/AppFooter.vue'
import CheckGroup from 'picocrank/vue/components/CheckGroup.vue'
import FormField from 'picocrank/vue/components/FormField.vue'
import FormLayout from 'picocrank/vue/components/FormLayout.vue'
import QuickSearch from 'picocrank/vue/components/QuickSearch.vue'
import RadioGroup from 'picocrank/vue/components/RadioGroup.vue'
import Section from 'picocrank/vue/components/Section.vue'
import Tabs from 'picocrank/vue/components/Tabs.vue'
import { useCollectionSourceForm } from '../composables/useCollectionSourceForm'

const tabsRef = ref<{ setActiveTab: (id: string) => void } | null>(null)

const {
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
} = useCollectionSourceForm()

const scheduleCronDescription = computed(
  () =>
    `${formCronSummary.value} Use standard 5-field cron when set. Empty cron is allowed: collectors only run the source when you use Run now from the list or detail page.`,
)

onMounted(() => {
  void initializeForm()
})

watch(pendingFocusTab, (tabId) => {
  if (tabId && tabsRef.value) {
    tabsRef.value.setActiveTab(tabId)
    pendingFocusTab.value = null
  }
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
        :title="pageTitle"
        :icon="DatabaseIcon"
        subtitle="Configure what to collect, how to identify the target, and when collectors should run."
      >
        <p v-if="loadErr" class="inline-notification error">{{ loadErr }}</p>
        <p v-else-if="loading" class="muted">Loading…</p>
        <div v-else class="collection-source-form">
          <Tabs ref="tabsRef" :tabs="formTabs" default-tab="source" :padding="true">
            <template #tab-source>
              <FormLayout>
                <FormField label="Collector type" for="form-collector-type" :error="collectorTypeError">
                  <select
                    id="form-collector-type"
                    v-model="formCollectorType"
                    class="mono"
                    @change="onFormCollectorTypeChange"
                  >
                    <option v-if="formCollectorTypeOptions.length === 0" disabled value="">
                      No collector-* heartbeats
                    </option>
                    <option v-for="t in formCollectorTypeOptions" :key="t" :value="t">
                      {{ t }}
                    </option>
                  </select>
                </FormField>
                <p v-if="formCollectorTypeOptions.length === 0" class="form-hint">
                  Start a <code>collector-*</code> service so its heartbeat appears in this list.
                </p>
                <p v-if="oneOffMode" class="form-hint">
                  One-off runs publish a <code>CollectionRequest</code> with an inline source. Nothing is persisted
                  and run history is not tied to a source id.
                </p>
                <template v-if="!oneOffMode && showStructuredSourceSpec && activeParsedSchema && formSchemaBuilder">
                  <FormField
                    label="Source pattern"
                    :component-has-label="true"
                    :error="sourcePatternError"
                  >
                    <RadioGroup
                      :model-value="formSchemaBuilder.patternId"
                      name="source-pattern"
                      variant="list"
                      aria-label="Source pattern"
                      :options="schemaPatternOptions"
                      @update:model-value="onSchemaPatternChange"
                    />
                  </FormField>
                  <FormField
                    v-for="a in activePatternArgs"
                    :key="a.id"
                    :label="a.label"
                    :for="`schema-arg-${a.id}`"
                    :error="schemaArgErrors[a.id] ?? ''"
                  >
                    <input
                      :id="`schema-arg-${a.id}`"
                      v-model="formSchemaBuilder.args[a.id]"
                      class="mono"
                      type="text"
                      autocomplete="off"
                    />
                  </FormField>
                  <FormField
                    v-if="activeParsedSchema.modifiers?.length"
                    label="Include"
                    :component-has-label="true"
                  >
                    <CheckGroup
                      v-model="selectedSchemaModifierIds"
                      name="source-modifiers"
                      aria-label="Source modifiers"
                      :options="schemaModifierOptions"
                    />
                  </FormField>
                </template>
                <FormField
                  v-if="showStructuredSourceSpec"
                  label="Source spec"
                  for="form-source-spec-composed"
                  :description="'Composed from the pattern, arguments, and modifiers above.'"
                  :error="sourceSpecError"
                >
                  <input
                    id="form-source-spec-composed"
                    v-model="formSourceSpec"
                    class="mono"
                    type="text"
                    readonly
                  />
                </FormField>
                <div
                  v-if="
                    !showStructuredSourceSpec ||
                    formSourceSpecAdvanced ||
                    (!oneOffMode && canUseStructuredForm)
                  "
                  class="source-spec-raw"
                >
                  <button
                    v-if="!oneOffMode && canUseStructuredForm"
                    type="button"
                    class="tiny neutral"
                    @click="toggleSourceSpecRaw"
                  >
                    {{ formSourceSpecAdvanced ? 'Use structured form' : 'Edit raw source spec' }}
                  </button>
                  <FormField
                    v-if="!showStructuredSourceSpec || formSourceSpecAdvanced"
                    label="Source spec"
                    for="form-source-spec"
                    :error="sourceSpecError"
                  >
                    <input
                      id="form-source-spec"
                      v-model="formSourceSpec"
                      class="mono"
                      type="text"
                      :placeholder="'e.g. org.repos:jamesread +dependabot +pr'"
                    />
                  </FormField>
                </div>
              </FormLayout>
            </template>

            <template v-if="!oneOffMode" #tab-schedule>
              <FormLayout>
                <FormField
                  label="Cron schedule (optional)"
                  for="form-cron-line"
                  :description-above="activeSchemaCronHint?.description ?? ''"
                  :description="scheduleCronDescription"
                  :error="cronLineError"
                >
                  <input
                    id="form-cron-line"
                    v-model="formCronLine"
                    class="mono cron-input"
                    type="text"
                    placeholder="empty = run now only, or e.g. 0 0 * * *"
                    spellcheck="false"
                  />
                  <div class="cron-presets">
                    <span class="presets-label">Presets:</span>
                    <button type="button" class="tiny neutral" @click="applyCronPreset('0 0 * * *')">
                      Daily midnight
                    </button>
                    <button type="button" class="tiny neutral" @click="applyCronPreset('0 * * * *')">Hourly</button>
                    <button type="button" class="tiny neutral" @click="applyCronPreset('*/15 * * * *')">
                      Every 15 min
                    </button>
                    <button type="button" class="tiny neutral" @click="applyCronPreset('0 0 * * 0')">
                      Weekly (Sun 00:00)
                    </button>
                  </div>
                </FormField>
                <FormField label="Enabled" for="form-enabled">
                  <input id="form-enabled" v-model="formEnabled" type="checkbox" />
                </FormField>
              </FormLayout>
            </template>
          </Tabs>

          <p v-if="formErr" class="inline-notification error">{{ formErr }}</p>

          <fieldset class="form-actions">
            <template v-if="oneOffMode">
              <button type="button" class="neutral" :disabled="saving" @click="cancel">Cancel</button>
              <button type="button" class="good" :disabled="saving || runNowPending" @click="onFormSubmit">
                {{ saving ? 'Working…' : 'Run once without saving' }}
              </button>
            </template>
            <template v-else>
              <button type="button" class="neutral" :disabled="saving" @click="cancel">Cancel</button>
              <button type="button" class="good" :disabled="saving" @click="onFormSubmit">
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
            </template>
          </fieldset>
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
.main {
  flex: 1;
  padding: 1rem 1.5rem 2rem;
}
.muted {
  color: #64748b;
}
.form-hint {
  margin: 0;
  font-size: 0.85rem;
  color: #64748b;
  line-height: 1.45;
}
.source-spec-raw {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
.cron-input {
  font-size: 0.9rem;
}
.cron-presets {
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
.collection-source-form :deep(form) {
  margin: 0;
}
.form-actions {
  margin-top: 0.75rem;
}
</style>
