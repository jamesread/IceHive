<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { create } from '@bufbuild/protobuf'
import { ConnectError } from '@connectrpc/connect'
import AppHeader from '../components/AppHeader.vue'
import AppFooter from '../components/AppFooter.vue'
import QuickSearch from 'picocrank/vue/components/QuickSearch.vue'
import Navigation from 'picocrank/vue/components/Navigation.vue'
import NavigationGrid from 'picocrank/vue/components/NavigationGrid.vue'
import Section from 'picocrank/vue/components/Section.vue'
import Tabs from 'picocrank/vue/components/Tabs.vue'
import { getControllerClient } from '../api/controllerClient'
import { ListServicesRequestSchema, type ServiceStatus } from '../gen/icehive/v1/controller_pb'

/** Landing view — links to Controller-backed tools. */
const router = useRouter()
const navigation = ref<any>(null)
const services = ref<ServiceStatus[]>([])
const loadErr = ref<string | null>(null)
const archHost = ref<HTMLElement | null>(null)
const archErr = ref<string | null>(null)

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

function heartbeatRowClass(status: string): string {
  return `hb-row hb-row--${normalizeHeartbeatStatus(status)}`
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
  try {
    const res = await getControllerClient().listServices(create(ListServicesRequestSchema, {}))
    services.value = [...res.services]
  } catch (e) {
    loadErr.value = e instanceof ConnectError ? e.message : String(e)
  } finally {
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
  navigation.value?.clearNavigationLinks()
  navigation.value?.addRouterLink('config', 'Controller configuration', {
    description: 'View and edit configuration variables via Controller Connect RPC',
  })
  navigation.value?.addRouterLink('sources', 'Collection sources', {
    description: 'Define collector targets and poll intervals (opaque specs per collector)',
  })
  navigation.value?.addCallback(
    'One-off collection',
    () => {
      void router.push({ name: 'sources', query: { oneOff: '1' } })
    },
    {
      name: 'one-off-collection',
      description: 'Enqueue a collection run immediately without persisting a source',
    },
  )
  void loadServices()
})
</script>

<template>
  <div class="shell">
    <AppHeader>
      <template #toolbar>
        <QuickSearch placeholder="Quick search..." />
      </template>
    </AppHeader>
    <main class="welcome">
      <Section title="Home">
        <p>This UI talks to the Controller over Connect RPC.</p>
        <Navigation ref="navigation">
          <NavigationGrid />
        </Navigation>
      </Section>
      <Section title="Services">
        <Tabs :tabs="[...statusTabs]" default-tab="heartbeats" @tab-change="onStatusTabChange">
          <template #tab-heartbeats>
            <p v-if="loadErr" class="err">{{ loadErr }}</p>
            <template v-else>
              <ul class="hb-legend" aria-label="Heartbeat status legend">
                <li><span class="pill pill-healthy">healthy</span> heartbeat within 30s</li>
                <li><span class="pill pill-stale">stale</span> older than 30s</li>
                <li><span class="pill pill-unknown">unknown</span> no heartbeat yet</li>
              </ul>
              <table class="svc-table">
                <thead>
                  <tr>
                    <th>Service</th>
                    <th>Status</th>
                    <th>Latest heartbeat</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="svc in services" :key="svc.serviceName" :class="heartbeatRowClass(svc.status)">
                    <td class="svc-name">{{ svc.serviceName }}</td>
                    <td>
                      <span class="pill" :class="`pill-${normalizeHeartbeatStatus(svc.status)}`">{{ svc.status }}</span>
                    </td>
                    <td class="mono">{{ heartbeatRelativeTime(svc.latestHeartbeatUnixMs) }}</td>
                  </tr>
                  <tr v-if="services.length === 0">
                    <td colspan="3">No service heartbeats yet.</td>
                  </tr>
                </tbody>
              </table>
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
.welcome {
  flex: 1;
  padding: 1rem 1.5rem 2rem;
}
.welcome :deep(.tab-panel) {
  padding: 0.75rem 0 0;
}
.svc-table {
  width: 100%;
  border-collapse: collapse;
}
.svc-table th,
.svc-table td {
  padding: 0.45rem 0.6rem;
  border-bottom: 1px solid #e2e8f0;
  text-align: left;
}
.err {
  color: #b91c1c;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
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
.pill {
  display: inline-block;
  padding: 0.12rem 0.45rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: none;
  letter-spacing: 0.01em;
}
.pill-healthy {
  background: #dcfce7;
  color: #14532d;
  border: 1px solid #86efac;
}
.pill-stale {
  background: #fef3c7;
  color: #92400e;
  border: 1px solid #fcd34d;
}
.pill-unknown {
  background: #f1f5f9;
  color: #475569;
  border: 1px solid #cbd5e1;
}

.hb-row--healthy {
  background: linear-gradient(90deg, #22c55e 3px, #f0fdf4 0);
}
.hb-row--stale {
  background: linear-gradient(90deg, #f59e0b 3px, #fffbeb 0);
}
.hb-row--unknown {
  background: linear-gradient(90deg, #94a3b8 3px, #f8fafc 0);
}
.hb-row td {
  background: transparent;
}
.svc-name {
  font-weight: 500;
}
</style>
