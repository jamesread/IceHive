import { createRouter, createWebHistory } from 'vue-router'
import { Configuration01Icon, DatabaseIcon } from '@hugeicons/core-free-icons'
import HomeView from '../views/HomeView.vue'
import ConfigView from '../views/ConfigView.vue'
import CollectionSourcesView from '../views/CollectionSourcesView.vue'
import CollectorDetailsView from '../views/CollectorDetailsView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'home',
      component: HomeView,
      meta: {
        title: 'Home',
        breadcrumbs: () => [{ name: 'Home', href: '/' }],
      },
    },
    {
      path: '/config',
      name: 'config',
      component: ConfigView,
      meta: {
        title: 'Controller configuration',
        icon: Configuration01Icon,
        breadcrumbs: () => [
          { name: 'Home', href: '/' },
          { name: 'Controller configuration', href: '/config' },
        ],
      },
    },
    {
      path: '/sources',
      name: 'sources',
      component: CollectionSourcesView,
      meta: {
        title: 'Collection sources',
        icon: DatabaseIcon,
        breadcrumbs: () => [
          { name: 'Home', href: '/' },
          { name: 'Collection sources', href: '/sources' },
        ],
      },
    },
    {
      path: '/sources/:id',
      name: 'collector-details',
      component: CollectorDetailsView,
      meta: {
        title: 'Collection source',
        breadcrumbs: (route: { params: { id?: string | string[] } }) => {
          const raw = route.params.id
          const id = Array.isArray(raw) ? raw[0] : raw
          const short = id && id.length > 10 ? `${id.slice(0, 8)}…` : id || 'Details'
          return [
            { name: 'Home', href: '/' },
            { name: 'Collection sources', href: '/sources' },
            { name: short, href: id ? `/sources/${id}` : '/sources' },
          ]
        },
      },
    },
  ],
})

router.afterEach((to) => {
  const pageTitle = typeof to.meta.title === 'string' ? to.meta.title : 'Home'
  document.title = `${pageTitle} » IceHive`
})

export default router
