<script setup lang="ts">
import { controllerVersion } from '../api/controllerClient'
</script>

<template>
  <footer class="app-footer">
    <span>IceHive</span>
    <span v-if="controllerVersion" class="app-footer-version">v{{ controllerVersion }}</span>
    <span><a href="https://github.com/jamesread/icehive">GitHub</a></span>
    <span><a href="https://jamesread.github.io/icehive/">Docs</a></span>
  </footer>
</template>

<style scoped>
.app-footer {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  padding: 0.65rem 1rem;
  border-top: 1px solid #e2e8f0;
  font-size: 0.8125rem;
  color: #64748b;
}
.app-footer-version {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
.app-footer a {
  color: inherit;
}
</style>
