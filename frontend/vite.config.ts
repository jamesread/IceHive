import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/icehive.v1.ControllerService': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
})
