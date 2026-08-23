import { showNotificationPopup } from 'picocrank/vue/composables/useNotificationPopups.js'

export function notifySuccess(message: string, label = 'Done'): void {
  showNotificationPopup({ message, label, class: 'good' })
}

export function notifyInfo(message: string, label = 'Note'): void {
  showNotificationPopup({ message, label, class: 'info' })
}

export function notifyError(message: string, label = 'Error'): void {
  showNotificationPopup({ message, label, class: 'bad' })
}
