<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { create } from '@bufbuild/protobuf'
import { ConnectError } from '@connectrpc/connect'
import AppHeader from '../components/AppHeader.vue'
import AppFooter from '../components/AppFooter.vue'
import { Activity01Icon, FlashIcon } from '@hugeicons/core-free-icons'
import QuickSearch from 'picocrank/vue/components/QuickSearch.vue'
import Navigation from 'picocrank/vue/components/Navigation.vue'
import NavigationGrid from 'picocrank/vue/components/NavigationGrid.vue'
import Section from 'picocrank/vue/components/Section.vue'
import { getControllerClient } from '../api/controllerClient'
import {
  ListActivityRequestSchema,
  ListCollectionSourcesRequestSchema,
  type ActivityEvent,
  type CollectionSource,
} from '../gen/icehive/v1/controller_pb'

/** Landing view — links to Controller-backed tools. */
const router = useRouter()
const navigation = ref<any>(null)
const activityEvents = ref<ActivityEvent[]>([])
const activityErr = ref<string | null>(null)
const activityLive = ref(false)
let navRefreshTimer: ReturnType<typeof setInterval> | null = null
let activityTimer: ReturnType<typeof setInterval> | null = null

/** Matches CollectionSourcesView pipeline status chip: disabled | stale | healthy. */
function pipelineHealthLabel(s: CollectionSource): string {
  if (!s.enabled) return 'disabled'
  if (s.isStale) return 'stale'
  return 'healthy'
}

function countBadCollectionSources(sources: CollectionSource[]): number {
  return sources.filter((s) => {
    const status = pipelineHealthLabel(s)
    return status !== 'healthy' && status !== 'disabled'
  }).length
}

function activityKindLabel(kind: string): string {
  switch (kind) {
    case 'heartbeat_received':
      return 'heartbeat'
    case 'collection_enqueued':
      return 'enqueue'
    case 'collection_run':
      return 'run'
    default:
      return kind || 'event'
  }
}

function activityKindKarma(ev: ActivityEvent): string {
  if (ev.kind === 'collection_run') {
    if (ev.success === true) return 'good'
    if (ev.success === false) return 'bad'
  }
  if (ev.kind === 'collection_enqueued') return 'note'
  return 'neutral'
}

function formatActivityTime(unixMs: bigint): string {
  const n = Number(unixMs)
  if (!Number.isFinite(n) || n <= 0) return '—'
  return new Date(n).toLocaleTimeString()
}

async function refreshCollectionSourcesNavCount() {
  if (!navigation.value) return
  try {
    const res = await getControllerClient().listCollectionSources(
      create(ListCollectionSourcesRequestSchema, { collectorType: '' }),
    )
    const badCount = countBadCollectionSources(res.sources)
    navigation.value.addRouterLink('sources', 'Collection sources', {
      description: 'Define collector targets and poll intervals (opaque specs per collector)',
      count: badCount,
    })
  } catch {
    // Keep the existing tile; other home content should still work.
  }
}

async function refreshActivity() {
  try {
    const res = await getControllerClient().listActivity(
      create(ListActivityRequestSchema, { limit: 50 }),
    )
    activityEvents.value = [...res.events]
    activityErr.value = null
    activityLive.value = true
  } catch (e) {
    activityErr.value = e instanceof ConnectError ? e.message : String(e)
    activityLive.value = false
  }
}

onMounted(() => {
  navigation.value?.clearNavigationLinks()
  navigation.value?.addRouterLink('sources', 'Collection sources', {
    description: 'Define collector targets and poll intervals (opaque specs per collector)',
  })
  navigation.value?.addRouterLink('services', 'Service heartbeats', {
    description: 'Live worker presence and architecture overview from Controller heartbeats',
  })
  navigation.value?.addCallback(
    'One-off collection',
    () => {
      void router.push({ name: 'sources', query: { oneOff: '1' } })
    },
    {
      name: 'one-off-collection',
      icon: FlashIcon,
      description: 'Enqueue a collection run immediately without persisting a source',
    },
  )
  navigation.value?.addRouterLink('config', 'Controller configuration', {
    description: 'View and edit configuration variables via Controller Connect RPC',
  })
  void refreshCollectionSourcesNavCount()
  void refreshActivity()
  navRefreshTimer = setInterval(() => {
    void refreshCollectionSourcesNavCount()
  }, 30_000)
  activityTimer = setInterval(() => {
    void refreshActivity()
  }, 3_000)
})

onUnmounted(() => {
  if (navRefreshTimer !== null) {
    clearInterval(navRefreshTimer)
    navRefreshTimer = null
  }
  if (activityTimer !== null) {
    clearInterval(activityTimer)
    activityTimer = null
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
    <main class="welcome">
      <Section title="Home">
        <p>This UI talks to the Controller over Connect RPC.</p>
        <Navigation ref="navigation">
          <NavigationGrid />
        </Navigation>
      </Section>

      <Section
        title="Live activity"
        :icon="Activity01Icon"
        subtitle="Recent heartbeats, collection publishes, and run reports observed by the Controller."
      >
        <template #toolbar>
          <span class="live-indicator" :class="{ on: activityLive }" title="Polling every 3s">
            <span class="live-dot" aria-hidden="true" />
            {{ activityLive ? 'Live' : 'Offline' }}
          </span>
        </template>

        <p v-if="activityErr" class="err" role="alert">{{ activityErr }}</p>
        <p v-else-if="activityEvents.length === 0" class="inline-notification note">
          Waiting for heartbeats and collection traffic…
        </p>
        <ul v-else class="activity-feed" aria-live="polite">
          <li v-for="ev in activityEvents" :key="ev.id" class="activity-row">
            <time class="activity-time mono" :datetime="new Date(Number(ev.unixMs)).toISOString()">
              {{ formatActivityTime(ev.unixMs) }}
            </time>
            <span class="tag" :class="activityKindKarma(ev)">{{ activityKindLabel(ev.kind) }}</span>
            <span class="activity-summary">{{ ev.summary }}</span>
          </li>
        </ul>
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
.err {
  color: #b91c1c;
  margin: 0;
}
.live-indicator {
  display: inline-flex;
  align-items: center;
  gap: 0.4em;
  font-size: 0.8125rem;
  color: #64748b;
  user-select: none;
}
.live-indicator.on {
  color: #15803d;
}
.live-dot {
  width: 0.55em;
  height: 0.55em;
  border-radius: 50%;
  background: #94a3b8;
}
.live-indicator.on .live-dot {
  background: #16a34a;
  animation: live-pulse 1.6s ease-in-out infinite;
}
@keyframes live-pulse {
  0%,
  100% {
    opacity: 1;
    transform: scale(1);
  }
  50% {
    opacity: 0.45;
    transform: scale(0.85);
  }
}
.activity-feed {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 22rem;
  overflow-y: auto;
}
.activity-row {
  display: grid;
  grid-template-columns: 5.5rem auto 1fr;
  gap: 0.65rem 0.75rem;
  align-items: baseline;
  padding: 0.45rem 0;
  border-bottom: 1px solid color-mix(in srgb, currentColor 12%, transparent);
  font-size: 0.875rem;
}
.activity-row:last-child {
  border-bottom: none;
}
.activity-time {
  color: #64748b;
  font-size: 0.8125rem;
}
.activity-summary {
  min-width: 0;
  overflow-wrap: anywhere;
  line-height: 1.4;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
</style>
