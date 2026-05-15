import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'
import ConfigView from '../views/ConfigView.vue'
import CollectionSourcesView from '../views/CollectionSourcesView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', name: 'home', component: HomeView, meta: { title: 'Home' } },
    { path: '/config', name: 'config', component: ConfigView, meta: { title: 'Controller configuration' } },
    { path: '/sources', name: 'sources', component: CollectionSourcesView, meta: { title: 'Collection sources' } },
  ],
})

router.afterEach((to) => {
  const pageTitle = typeof to.meta.title === 'string' ? to.meta.title : 'Home'
  document.title = `${pageTitle} » IceHive`
})

export default router
