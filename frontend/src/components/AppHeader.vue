<script setup lang="ts">
import { useRouter } from 'vue-router'
import Header from 'picocrank/vue/components/Header.vue'

const router = useRouter()

function onLogoClick() {
  void router.push({ name: 'home' })
}
</script>

<template>
  <Header
    title="IceHive"
    :sidebar-enabled="false"
    :show-branding="true"
    :breadcrumbs="true"
    :theme-toggle-enabled="true"
    logo-url="/favicon.svg"
    @logo-click="onLogoClick"
  >
    <template v-if="$slots.toolbar" #toolbar>
      <slot name="toolbar" />
    </template>
  </Header>
</template>
