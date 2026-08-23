<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import Header from 'picocrank/vue/components/Header.vue'
import { controllerVersion, getActiveControllerBaseUrl } from '../api/controllerClient'

const router = useRouter()

function onLogoClick() {
  void router.push({ name: 'home' })
}

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
  <Header
    title="IceHive"
    :sidebar-enabled="false"
    :show-branding="true"
    :breadcrumbs="true"
    logo-url="/favicon.svg"
    @logo-click="onLogoClick"
  >
    <template v-if="$slots.toolbar" #toolbar>
      <slot name="toolbar" />
    </template>
    <template #user-info>
      <span v-if="controllerBaseUrl || controllerVersion" class="controller-meta">
        <span
          v-if="controllerBaseUrl"
          class="annotation neutral controller-chip"
          :title="controllerBaseUrl"
        >
          <span class="annotation-key">controller</span>
          <span class="annotation-val mono">{{ shortControllerUrl }}</span>
        </span>
        <span v-if="controllerVersion" class="annotation neutral controller-chip">
          <span class="annotation-key">version</span>
          <span class="annotation-val mono">v{{ controllerVersion }}</span>
        </span>
      </span>
    </template>
  </Header>
</template>

<style scoped>
.controller-meta {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem;
}
.controller-chip {
  margin: 0;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.8125rem;
}
</style>
