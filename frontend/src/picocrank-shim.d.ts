declare module 'picocrank/vue/components/Header.vue' {
  import type { DefineComponent } from 'vue'

  const component: DefineComponent<object, object, unknown>
  export default component
}

declare module 'picocrank/vue/components/Navigation.vue' {
  import type { DefineComponent } from 'vue'

  const component: DefineComponent<object, object, unknown>
  export default component
}

declare module 'picocrank/vue/components/NavigationGrid.vue' {
  import type { DefineComponent } from 'vue'

  const component: DefineComponent<object, object, unknown>
  export default component
}

declare module 'picocrank/vue/components/Section.vue' {
  import type { DefineComponent } from 'vue'

  const component: DefineComponent<object, object, unknown>
  export default component
}

declare module 'picocrank/vue/components/QuickSearch.vue' {
  import type { DefineComponent } from 'vue'

  const component: DefineComponent<object, object, unknown>
  export default component
}

declare module 'picocrank/vue/components/Tabs.vue' {
  import type { DefineComponent } from 'vue'

  const component: DefineComponent<object, object, unknown>
  export default component
}

declare module 'picocrank/vue/components/Table.vue' {
  import type { DefineComponent } from 'vue'

  const component: DefineComponent<object, object, unknown>
  export default component
}

declare module 'picocrank/vue/components/NotificationPopups.vue' {
  import type { DefineComponent } from 'vue'

  const component: DefineComponent<object, object, unknown>
  export default component
}

declare module 'picocrank/vue/composables/useNotificationPopups.js' {
  export function showNotificationPopup(options?: {
    message?: string
    label?: string | null
    class?: string
    linkTo?: string | null
    linkLabel?: string | null
    durationMs?: number
    id?: string
  }): string | null

  export function useNotificationPopups(): {
    show: typeof showNotificationPopup
    dismiss: (id: string) => void
    dismissAll: () => void
  }
}
