/** Parse Kubernetes service-link style values into an HTTP upstream for the dev proxy. */
export function controllerProxyTargetFromEnv(
  raw: string | undefined,
  fallback = 'http://127.0.0.1:8080',
): string {
  const val = raw?.trim()
  if (!val) return fallback
  if (val.startsWith('tcp://')) return `http://${val.slice('tcp://'.length)}`
  if (/^https?:\/\//i.test(val)) return val.replace(/\/+$/, '')
  return `http://${val.replace(/\/+$/, '')}`
}
