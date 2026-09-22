<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { create } from '@bufbuild/protobuf'
import { ConnectError } from '@connectrpc/connect'
import { HugeiconsIcon } from '@hugeicons/vue'
import { Activity01Icon, ArrowReloadHorizontalIcon } from '@hugeicons/core-free-icons'
import AppHeader from '../components/AppHeader.vue'
import AppFooter from '../components/AppFooter.vue'
import QuickSearch from 'picocrank/vue/components/QuickSearch.vue'
import Section from 'picocrank/vue/components/Section.vue'
import Table from 'picocrank/vue/components/Table.vue'
import Tabs from 'picocrank/vue/components/Tabs.vue'
import { getControllerClient } from '../api/controllerClient'
import { ListServicesRequestSchema, type ServiceStatus } from '../gen/icehive/v1/controller_pb'

const services = ref<ServiceStatus[]>([])
const loadErr = ref<string | null>(null)
const loading = ref(false)
const archHost = ref<HTMLElement | null>(null)
const archErr = ref<string | null>(null)
let heartbeatTimer: ReturnType<typeof setInterval> | null = null

const statusTabs = [
  { id: 'heartbeats', label: 'Service heartbeats' },
  { id: 'architecture', label: 'Architecture' },
] as const

type HeartbeatUiStatus = 'healthy' | 'stale' | 'unknown'

function normalizeHeartbeatStatus(raw: string): HeartbeatUiStatus {
  const v = (raw || '').toLowerCase().trim()
  if (v === 'healthy') return 'healthy'
  if (v === 'stale') return 'stale'
  return 'unknown'
}

function heartbeatStatusKarma(status: HeartbeatUiStatus): string {
  if (status === 'healthy') return 'good'
  if (status === 'stale') return 'warning'
  return 'note'
}

function heartbeatRelativeTime(unixMs: bigint): string {
  const ts = Number(unixMs)
  if (!Number.isFinite(ts) || ts <= 0) return 'unknown'
  const diffMs = Date.now() - ts
  if (diffMs < 0) return 'just now'
  const sec = Math.floor(diffMs / 1000)
  if (sec < 60) return `${sec}s ago`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}m ago`
  const hr = Math.floor(min / 60)
  if (hr < 24) return `${hr}h ago`
  const day = Math.floor(hr / 24)
  return `${day}d ago`
}

const tableHeaders = [
  { key: 'serviceName', label: 'Service', sortable: true },
  { key: 'version', label: 'Version', sortable: true, width: '8rem' },
  { key: 'status', label: 'Status', sortable: true, width: '8rem' },
  { key: 'latestHeartbeat', label: 'Latest heartbeat', sortable: true, width: '10rem' },
]

const tableRows = computed(() =>
  services.value.map((svc) => ({
    serviceName: svc.serviceName,
    version: svc.version || '—',
    status: normalizeHeartbeatStatus(svc.status),
    latestHeartbeat: heartbeatRelativeTime(svc.latestHeartbeatUnixMs),
  })),
)

function buildArchitectureMermaid(liveServices: ServiceStatus[]): string {
  const controllerService = liveServices.find((s) => (s.serviceName ?? '').trim() === 'controller')
  const controllerStatus = normalizeHeartbeatStatus(controllerService?.status ?? '')

  const collectors = [...liveServices]
    .filter((s) => (s.serviceName ?? '').startsWith('collector-'))
    .sort((a, b) => (a.serviceName ?? '').localeCompare(b.serviceName ?? ''))

  const collectorNodes: string[] = []
  const collectorClasses: string[] = []
  if (collectors.length === 0) {
    collectorNodes.push('    COL0["No collector heartbeats"]')
    collectorClasses.push('COL0')
  } else {
    collectors.forEach((svc, idx) => {
      const id = `COL${idx + 1}`
      const name = svc.serviceName ?? 'collector-unknown'
      const status = normalizeHeartbeatStatus(svc.status)
      collectorNodes.push(`    ${id}["${name}<br/>(${status})"]`)
      collectorClasses.push(`${id} svcColl-${status}`)
    })
  }

  return `flowchart TB
  subgraph clients["Clients"]
    FE["Frontend Vue"]
  end
  subgraph control["Controller plane"]
    CTRL["Controller Connect API<br/>(${controllerStatus})"]
    CMETA[("MySQL<br/>metadata & config")]
  end
  subgraph bus["Messaging"]
    RMQ[("RabbitMQ<br/>topic exchange")]
  end
  subgraph coll["Collector workers (from heartbeats)"]
${collectorNodes.join('\n')}
  end
  subgraph pers["Persister workers"]
    PM["persister-mysql"]
    PY["persister-yaml"]
  end
  subgraph sinks["Sinks"]
    EMY[("MySQL<br/>entity tables")]
    YFS["YAML / files"]
  end
  subgraph ext["External systems"]
    API["HTTP APIs & feeds"]
  end

  FE -->|Connect RPC| CTRL
  CTRL --> CMETA
  coll -->|"WorkerBootstrap, list sources, report runs"| CTRL
  pers -->|WorkerBootstrap| CTRL
  coll -->|fetch| API
  coll -->|"publish entities and schemas"| RMQ
  RMQ -->|consume entities| pers
  PM --> EMY
  PY --> YFS

  classDef svcClient fill:#e0f2fe,stroke:#0369a1,stroke-width:2px,color:#0f172a
  classDef svcControl-healthy fill:#dcfce7,stroke:#15803d,stroke-width:2px,color:#0f172a
  classDef svcControl-stale fill:#fef3c7,stroke:#b45309,stroke-width:2px,color:#0f172a
  classDef svcControl-unknown fill:#f1f5f9,stroke:#64748b,stroke-width:2px,color:#0f172a
  classDef svcBus fill:#fce7f3,stroke:#be185d,stroke-width:2px,color:#0f172a
  classDef svcColl-healthy fill:#dcfce7,stroke:#15803d,stroke-width:2px,color:#0f172a
  classDef svcColl-stale fill:#fef3c7,stroke:#b45309,stroke-width:2px,color:#0f172a
  classDef svcColl-unknown fill:#f1f5f9,stroke:#64748b,stroke-width:2px,color:#0f172a
  classDef svcPers fill:#ede9fe,stroke:#6d28d9,stroke-width:2px,color:#0f172a
  classDef svcSink fill:#cffafe,stroke:#0891b2,stroke-width:2px,color:#0f172a
  classDef svcExt fill:#f1f5f9,stroke:#64748b,stroke-width:2px,color:#0f172a

  class FE svcClient
  class CTRL svcControl-${controllerStatus}
  class CMETA svcControl-unknown
  class RMQ svcBus
  class PM,PY svcPers
  class EMY,YFS svcSink
  class API svcExt
  ${collectorClasses.map((x) => `class ${x}`).join('\n  ')}`
}

async function loadServices() {
  loadErr.value = null
  loading.value = true
  try {
    const res = await getControllerClient().listServices(create(ListServicesRequestSchema, {}))
    services.value = [...res.services]
  } catch (e) {
    loadErr.value = e instanceof ConnectError ? e.message : String(e)
  } finally {
    loading.value = false
    await renderArchitectureDiagram()
  }
}

function onStatusTabChange(_tab: (typeof statusTabs)[number], tabId: string | number) {
  if (tabId === 'architecture') {
    void renderArchitectureDiagram()
  }
}

async function renderArchitectureDiagram() {
  archErr.value = null
  await nextTick()
  const el = archHost.value
  if (!el) {
    return
  }
  try {
    const { default: mermaid } = await import('mermaid')
    mermaid.initialize({
      startOnLoad: false,
      theme: 'neutral',
      securityLevel: 'strict',
      fontFamily: 'ui-sans-serif, system-ui, sans-serif',
    })
    const id = `icehive-arch-${Date.now()}`
    const { svg } = await mermaid.render(id, buildArchitectureMermaid(services.value))
    el.innerHTML = svg
  } catch (e) {
    archErr.value = e instanceof Error ? e.message : String(e)
  }
}

onMounted(() => {
  void loadServices()
  heartbeatTimer = setInterval(() => {
    void loadServices()
  }, 30_000)
})

onUnmounted(() => {
  if (heartbeatTimer !== null) {
    clearInterval(heartbeatTimer)
    heartbeatTimer = null
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
        title="Service heartbeats"
        :icon="Activity01Icon"
        subtitle="Live worker presence from the Controller, plus a diagram of how collectors, messaging, and sinks connect."
      >
        <template #toolbar>
          <button type="button" class="neutral" title="Refresh" :disabled="loading" @click="loadServices">
            <HugeiconsIcon :icon="ArrowReloadHorizontalIcon" width="1em" height="1em" aria-hidden="true" />
          </button>
        </template>

        <Tabs :tabs="[...statusTabs]" default-tab="heartbeats" @tab-change="onStatusTabChange">
          <template #tab-heartbeats>
            <p v-if="loadErr" class="err">{{ loadErr }}</p>
            <template v-else>
              <ul class="hb-legend" aria-label="Heartbeat status legend">
                <li><span class="tag good">healthy</span> heartbeat within 30s</li>
                <li><span class="tag warning">stale</span> older than 30s</li>
                <li><span class="tag note">unknown</span> no heartbeat yet</li>
              </ul>
              <p v-if="services.length === 0" class="inline-notification note">No service heartbeats yet.</p>
              <Table
                v-else
                class="row-hover"
                :headers="tableHeaders"
                :data="tableRows"
                row-key="serviceName"
                :show-pagination="false"
              >
                <template #cell-serviceName="{ value }">
                  <strong>{{ value }}</strong>
                </template>
                <template #cell-status="{ value }">
                  <span class="tag" :class="heartbeatStatusKarma(value)">{{ value }}</span>
                </template>
              </Table>
            </template>
          </template>
          <template #tab-architecture>
            <p class="arch-intro">
              Collectors normalize vendor data and publish to RabbitMQ; persisters write to sinks. Workers bootstrap AMQP
              and sink settings from the Controller; the UI uses the Controller API only.
            </p>
            <ul class="arch-legend" aria-label="Diagram color legend">
              <li><span class="swatch swatch-client" /> Clients</li>
              <li><span class="swatch swatch-control" /> Controller plane</li>
              <li><span class="swatch swatch-bus" /> Messaging</li>
              <li><span class="swatch swatch-coll" /> Collectors</li>
              <li><span class="swatch swatch-pers" /> Persisters</li>
              <li><span class="swatch swatch-sink" /> Sinks</li>
              <li><span class="swatch swatch-ext" /> External</li>
            </ul>
            <p v-if="archErr" class="err">{{ archErr }}</p>
            <div ref="archHost" class="mermaid-arch" aria-label="IceHive service architecture diagram" />
          </template>
        </Tabs>
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
.main :deep(.tab-panel) {
  padding: 0.75rem 0 0;
}
.err {
  color: #b91c1c;
}
.hb-legend {
  margin: 0 0 0.65rem;
  padding: 0;
  list-style: none;
  font-size: 0.8125rem;
  color: #475569;
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem 1.25rem;
}
.hb-legend li {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
}
.arch-intro {
  max-width: 52rem;
  margin: 0 0 0.75rem;
  color: #475569;
  line-height: 1.5;
}
.mermaid-arch {
  width: 100%;
  overflow-x: auto;
  padding: 0.5rem 0 1rem;
}
.mermaid-arch :deep(svg) {
  max-width: 100%;
  height: auto;
}

.arch-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 0.65rem 1.25rem;
  margin: 0 0 0.75rem;
  padding: 0;
  list-style: none;
  font-size: 0.8125rem;
  color: #475569;
}
.arch-legend li {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
}
.swatch {
  width: 0.75rem;
  height: 0.75rem;
  border-radius: 2px;
  border: 1px solid rgba(15, 23, 42, 0.2);
  flex-shrink: 0;
}
.swatch-client {
  background: #e0f2fe;
  border-color: #0369a1;
}
.swatch-control {
  background: #fef9c3;
  border-color: #ca8a04;
}
.swatch-bus {
  background: #fce7f3;
  border-color: #be185d;
}
.swatch-coll {
  background: #dcfce7;
  border-color: #15803d;
}
.swatch-pers {
  background: #ede9fe;
  border-color: #6d28d9;
}
.swatch-sink {
  background: #cffafe;
  border-color: #0891b2;
}
.swatch-ext {
  background: #f1f5f9;
  border-color: #64748b;
}
</style>
