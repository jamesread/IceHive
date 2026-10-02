import { ref } from 'vue'
import { createClient, type Client } from '@connectrpc/connect'
import { createConnectTransport } from '@connectrpc/connect-web'
import { create } from '@bufbuild/protobuf'
import { ConnectError } from '@connectrpc/connect'
import { ControllerService, InitRequestSchema } from '../gen/icehive/v1/controller_pb'

/** Previous single-URL session key; migrated into local history on read. */
const LEGACY_SESSION_KEY = 'icehive.controllerBaseUrl'
const HISTORY_STORAGE_KEY = 'icehive.controllerBaseUrlHistory'
const HISTORY_LIMIT = 5
const DEFAULT_TIMEOUT_MS = 10_000

let activeClient: Client<typeof ControllerService> | null = null
let activeBaseUrl = ''

/** Controller version from the most recent successful Init call. */
export const controllerVersion = ref<string | null>(null)

/** Non-fatal startup issues reported by Init (e.g. missing config.yaml). */
export const controllerStartupWarnings = ref<string[]>([])

function buildTransport(baseUrl: string) {
  return createConnectTransport({
    baseUrl,
    defaultTimeoutMs: DEFAULT_TIMEOUT_MS,
  })
}

export function getActiveControllerBaseUrl(): string {
  return activeBaseUrl
}

function readRawHistory(): string[] {
  try {
    const raw = localStorage.getItem(HISTORY_STORAGE_KEY)
    if (!raw) return []
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    const out: string[] = []
    for (const item of parsed) {
      if (typeof item !== 'string') continue
      const trimmed = item.trim()
      if (!trimmed) continue
      let canonical: string
      try {
        canonical = normalizeControllerBaseUrl(trimmed)
      } catch {
        continue
      }
      if (out.includes(canonical)) continue
      out.push(canonical)
      if (out.length >= HISTORY_LIMIT) break
    }
    return out
  } catch {
    return []
  }
}

function writeHistory(urls: string[]): void {
  try {
    if (urls.length === 0) localStorage.removeItem(HISTORY_STORAGE_KEY)
    else localStorage.setItem(HISTORY_STORAGE_KEY, JSON.stringify(urls.slice(0, HISTORY_LIMIT)))
  } catch {
    /* ignore quota / private mode */
  }
}

function migrateLegacySessionUrl(): string[] {
  let legacy = ''
  try {
    legacy = sessionStorage.getItem(LEGACY_SESSION_KEY)?.trim() ?? ''
  } catch {
    return []
  }
  if (!legacy) return []
  let canonical = legacy
  try {
    canonical = normalizeControllerBaseUrl(legacy)
  } catch {
    return []
  }
  writeHistory([canonical])
  try {
    sessionStorage.removeItem(LEGACY_SESSION_KEY)
  } catch {
    /* ignore */
  }
  return [canonical]
}

/** Newest first, at most {@link HISTORY_LIMIT} entries. */
export function readStoredControllerBaseUrls(): string[] {
  const stored = readRawHistory()
  if (stored.length > 0) return stored
  return migrateLegacySessionUrl()
}

export function readStoredControllerBaseUrl(): string | null {
  return readStoredControllerBaseUrls()[0] ?? null
}

/**
 * Remember a controller URL (newest first, capped at 5) or clear the history
 * when `url` is null or blank.
 */
export function persistStoredControllerBaseUrl(url: string | null): void {
  const trimmed = url?.trim() ?? ''
  if (!trimmed) {
    writeHistory([])
    try {
      sessionStorage.removeItem(LEGACY_SESSION_KEY)
    } catch {
      /* ignore */
    }
    return
  }
  let normalized: string
  try {
    normalized = normalizeControllerBaseUrl(trimmed)
  } catch {
    return
  }
  const next = [normalized, ...readStoredControllerBaseUrls().filter((entry) => entry !== normalized)]
  writeHistory(next.slice(0, HISTORY_LIMIT))
}

/** Normalize user or env input into a Connect base URL (no trailing slash). */
export function normalizeControllerBaseUrl(raw: string): string {
  const t = raw.trim().replace(/\/+$/, '')
  if (!t) throw new Error('Controller URL is empty')
  if (t.startsWith('/')) return t
  if (/^https?:\/\//i.test(t)) return t
  return `http://${t}`.replace(/\/+$/, '')
}

function uniquePush(list: string[], v: string): void {
  if (list.includes(v)) return
  list.push(v)
}

/** Same-origin Connect base URL (Vite dev proxy and production ingress both forward /api). */
export const SAME_ORIGIN_CONTROLLER_BASE_URL = '/api'

/**
 * Endpoints to try before prompting (order: saved override, build env, same-origin /api,
 * then host:8080).
 */
export function controllerBaseUrlCandidates(): string[] {
  const out: string[] = []

  const stored = readStoredControllerBaseUrl()
  if (stored) uniquePush(out, stored)

  const env = import.meta.env.VITE_CONTROLLER_BASE_URL?.trim()
  if (env) {
    try {
      uniquePush(out, normalizeControllerBaseUrl(env))
    } catch {
      /* invalid env — skip */
    }
  }

  uniquePush(out, SAME_ORIGIN_CONTROLLER_BASE_URL)

  if (typeof window !== 'undefined') {
    uniquePush(out, `${window.location.protocol}//${window.location.hostname}:8080`)
  }

  return out
}

function displayCandidate(raw: string): string {
  return raw === SAME_ORIGIN_CONTROLLER_BASE_URL ? '(same-origin /api)' : raw
}

function resolveProbeBaseUrl(raw: string): string {
  return normalizeControllerBaseUrl(raw)
}

async function probeController(rawBaseUrl: string): Promise<{ version: string; warnings: string[] }> {
  const baseUrl = resolveProbeBaseUrl(rawBaseUrl)
  const transport = buildTransport(baseUrl)
  const client = createClient(ControllerService, transport)
  const res = await client.init(create(InitRequestSchema, {}))
  const version = res.version?.trim()
  if (!version) throw new Error('Init returned empty version')
  const warnings = (res.startupWarnings ?? []).map((w) => w.trim()).filter(Boolean)
  return { version, warnings }
}

export function assignControllerClient(rawBaseUrl: string): void {
  activeBaseUrl = normalizeControllerBaseUrl(rawBaseUrl)
  activeClient = createClient(ControllerService, buildTransport(activeBaseUrl))
}

export function getControllerClient(): Client<typeof ControllerService> {
  if (!activeClient) throw new Error('Controller client is not initialized')
  return activeClient
}

export type ControllerConnectResult =
  | { ok: true; baseUrl: string }
  | { ok: false; attempted: string[]; lastError: string }

export async function connectToFirstAvailableController(): Promise<ControllerConnectResult> {
  controllerVersion.value = null
  controllerStartupWarnings.value = []
  const attempted: string[] = []
  let lastError = 'No connection candidates'
  for (const raw of controllerBaseUrlCandidates()) {
    attempted.push(displayCandidate(raw))
    try {
      const { version, warnings } = await probeController(raw)
      const normalized = normalizeControllerBaseUrl(raw)
      assignControllerClient(normalized)
      controllerVersion.value = version
      controllerStartupWarnings.value = warnings
      return { ok: true, baseUrl: normalized }
    } catch (e) {
      lastError =
        e instanceof ConnectError ? e.message : e instanceof Error ? e.message : String(e)
    }
  }
  return { ok: false, attempted, lastError }
}

/** Probe, install global client, and optionally persist for the next visit. */
export async function connectWithUserSuppliedBaseUrl(raw: string, persist: boolean): Promise<void> {
  const normalized = normalizeControllerBaseUrl(raw)
  const { version, warnings } = await probeController(normalized)
  assignControllerClient(normalized)
  controllerVersion.value = version
  controllerStartupWarnings.value = warnings
  persistStoredControllerBaseUrl(persist ? normalized : null)
}
