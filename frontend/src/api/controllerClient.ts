import { createClient, type Client } from '@connectrpc/connect'
import { createConnectTransport } from '@connectrpc/connect-web'
import { create } from '@bufbuild/protobuf'
import { ConnectError } from '@connectrpc/connect'
import {
  ControllerService,
  ListServicesRequestSchema,
} from '../gen/icehive/v1/controller_pb'

const STORAGE_KEY = 'icehive.controllerBaseUrl'
const DEFAULT_TIMEOUT_MS = 10_000

let activeClient: Client<typeof ControllerService> | null = null
let activeBaseUrl = ''

function buildTransport(baseUrl: string) {
  return createConnectTransport({
    baseUrl,
    defaultTimeoutMs: DEFAULT_TIMEOUT_MS,
  })
}

export function getActiveControllerBaseUrl(): string {
  return activeBaseUrl
}

export function readStoredControllerBaseUrl(): string | null {
  try {
    const v = sessionStorage.getItem(STORAGE_KEY)?.trim()
    return v || null
  } catch {
    return null
  }
}

export function persistStoredControllerBaseUrl(url: string | null): void {
  try {
    if (url?.trim()) sessionStorage.setItem(STORAGE_KEY, url.trim())
    else sessionStorage.removeItem(STORAGE_KEY)
  } catch {
    /* ignore quota / private mode */
  }
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

async function probeController(rawBaseUrl: string): Promise<void> {
  const baseUrl = resolveProbeBaseUrl(rawBaseUrl)
  const transport = buildTransport(baseUrl)
  const client = createClient(ControllerService, transport)
  await client.listServices(create(ListServicesRequestSchema, {}))
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
  const attempted: string[] = []
  let lastError = 'No connection candidates'
  for (const raw of controllerBaseUrlCandidates()) {
    attempted.push(displayCandidate(raw))
    try {
      await probeController(raw)
      const normalized = normalizeControllerBaseUrl(raw)
      assignControllerClient(normalized)
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
  await probeController(normalized)
  assignControllerClient(normalized)
  persistStoredControllerBaseUrl(persist ? normalized : null)
}
