import 'femtocrank/style.css'
import 'picocrank/styles.css'
import 'picocrank/vue/composables/useTheme.js'

import { createApp } from 'vue'
import App from './App.vue'
import router from './router'

createApp(App).use(router).mount('#app')
