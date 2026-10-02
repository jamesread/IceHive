export type NotificationPopupOptions = {
  message?: string
  label?: string | null
  class?: string
  linkTo?: string | null
  linkLabel?: string | null
  durationMs?: number
  id?: string
}

type ShowNotificationPopup = (options?: NotificationPopupOptions) => string | null

let showNotificationPopupImpl: ShowNotificationPopup | null = null

/** Bind to `<NotificationPopups ref="…" />` so toasts share the mounted popup stack. */
export function registerNotificationPopupShow(show: ShowNotificationPopup): void {
  showNotificationPopupImpl = show
}

function showPopup(options: NotificationPopupOptions): void {
  if (!showNotificationPopupImpl) {
    console.warn('[notify] NotificationPopups is not registered yet.')
    return
  }
  showNotificationPopupImpl(options)
}

export function notifySuccess(message: string, label = 'Done'): void {
  showPopup({ message, label, class: 'good' })
}

export function notifyRunEnqueued(message = 'Collection run enqueued.'): void {
  showPopup({ message, label: 'Run now', class: 'good' })
}

export function notifyInfo(message: string, label = 'Note'): void {
  showPopup({ message, label, class: 'info' })
}

export function notifyError(message: string, label = 'Error'): void {
  showPopup({ message, label, class: 'bad' })
}
