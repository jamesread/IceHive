import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { controllerProxyTargetFromEnv } from './vite.controllerProxy'

const controllerProxyTarget = controllerProxyTargetFromEnv(process.env.ICEHIVE_CONTROLLER_PORT)

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/api': {
        target: controllerProxyTarget,
        changeOrigin: true,
      },
    },
  },
})
