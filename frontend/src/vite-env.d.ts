/// <reference types="vite/client" />

interface ImportMetaEnv {
  /**
   * Base URL of the Controller HTTP server (e.g. http://127.0.0.1:8080).
   * When set, tried early during startup (after any saved session URL).
   * When unset, the app probes Vite same-origin in dev, then `location` host port 8080, then same-origin in production.
   */
  readonly VITE_CONTROLLER_BASE_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
