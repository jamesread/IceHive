<script setup lang="ts">
import { computed } from 'vue'
import { controllerVersion, getActiveControllerBaseUrl } from '../api/controllerClient'

const controllerBaseUrl = computed(() => getActiveControllerBaseUrl())

const shortControllerUrl = computed(() => {
  const raw = controllerBaseUrl.value
  if (!raw) return ''
  if (raw === '/api') return '/api'
  try {
    const u = new URL(raw)
    const host = u.port ? `${u.hostname}:${u.port}` : u.hostname
    return host.length > 28 ? `${host.slice(0, 26)}…` : host
  } catch {
    return raw.length > 28 ? `${raw.slice(0, 26)}…` : raw
  }
})
</script>

<template>
  <footer class="app-footer">
    <span>IceHive</span>
    <span v-if="controllerVersion">v{{ controllerVersion }}</span>
    <span
      v-if="controllerBaseUrl"
      class="app-footer-controller"
      :title="controllerBaseUrl"
    >
      {{ shortControllerUrl }}
    </span>
    <span><a href="https://github.com/jamesread/icehive">GitHub</a></span>
    <span><a href="https://jamesread.github.io/icehive/">Docs</a></span>
  </footer>
</template>

<style scoped>
.app-footer {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-wrap: wrap;
  gap: 0.5rem;
  padding: 0.65rem 1rem;
  border-top: 1px solid #e2e8f0;
  font-size: 0.8125rem;
  color: #64748b;
}
.app-footer-controller {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.app-footer a {
  color: inherit;
}
</style>
