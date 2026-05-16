# collector-jmap

The **JMAP** collector polls **`collection_sources`**, honours cron schedules, executes read-only mailbox queries via JMAP Session/API endpoints, and publishes normalized entities downstream with the **same housekeeping knobs documented for `collector-github`** (metrics sidecar cadence fields, cron semantics, heartbeat publishing, …).

Mailbox-level secrets are presently supplied **only through environment variables** (see below). Mailboxes/`source_unique_id` payloads still flow from relational **`collection_sources.source_spec`** (plus optional **`ICEHIVE_JMAP_MAILBOX_ID`** shortcuts during early bring-up).

## Container invocation

| Setting | Value |
|---------|-------|
| `ICEHIVE_SERVICE` | `collector-jmap` |
| `ICEHIVE_CONTROLLER_URL` | Controller base URL |

```
-configdir /etc/icehive
```

Default **`/healthz`** bind **`-listen :8086`** unless overridden.

### JMAP environment variables

Kubernetes should inject these via **Secrets**/`envFrom`:

| Env | Requirement | Purpose |
|-----|--------------|---------|
| `ICEHIVE_JMAP_BEARER_TOKEN` or `ICEHIVE_JMAP_API_KEY` | Provide **exactly one** bearer-style secret (`API_KEY` is legacy naming) | `Authorization` header for session + invoke calls |
| `ICEHIVE_JMAP_SESSION_URL` or `ICEHIVE_JMAP_CONNECT_URL` | Required unless **`ICEHIVE_JMAP_API_URL`** points directly at the POST API | `GET` target that returns RFC 8620 Session JSON (**`CONNECT_URL`** is an accepted alias discovered in deployments) |
| `ICEHIVE_JMAP_API_URL` | Alternate path when skipping session bootstrap | Explicit POST endpoint (must not be only the GET session document URL—controller logs warn on HTTP 405 regressions) |
| `ICEHIVE_JMAP_ACCOUNT_ID` | Optional when session JSON lists a primary mail account | Pin the account UUID when automation cannot derive it automatically |
| `ICEHIVE_JMAP_MAILBOX_ID` | Optional | Forces a specific mailbox when **`source_spec` omits identifiers** |

## Optional YAML (`collector-jmap.yaml`)

| Key | Default | Purpose |
|-----|---------|---------|
| `listen` | `:8086` | Metrics/health HTTP bind (**overrides `-listen`** when set) |
| `poll_interval_seconds` | `60` | Seconds between scans of relational collection recipes |

## Routing keys

Announcement traffic uses **`collector.source_schema.collector-jmap`**; on-demand jobs enqueue with **`collector.collection_request.collector-jmap`**.
