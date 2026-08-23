const DEFAULT_INTERVAL_MS = 3000
const DEFAULT_MAX_ATTEMPTS = 10

/** Poll until last_run_unix_ms advances past beforeRunMs, or attempts are exhausted. */
export async function pollAfterCollectionRun(
  beforeRunMs: bigint,
  reload: () => Promise<void>,
  readLastRunMs: () => bigint | undefined,
  opts?: { intervalMs?: number; maxAttempts?: number },
): Promise<boolean> {
  const intervalMs = opts?.intervalMs ?? DEFAULT_INTERVAL_MS
  const maxAttempts = opts?.maxAttempts ?? DEFAULT_MAX_ATTEMPTS
  for (let i = 0; i < maxAttempts; i++) {
    await new Promise((resolve) => setTimeout(resolve, intervalMs))
    await reload()
    const lastRun = readLastRunMs()
    if (lastRun !== undefined && lastRun > beforeRunMs) {
      return true
    }
  }
  return false
}
