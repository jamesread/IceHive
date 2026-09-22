# AGENTS.md

Guide for AI agents integrating with IceHive.

## Project overview

IceHive collects data from vendor APIs, normalizes it, and stores it via collectors, RabbitMQ, and persisters. The **Controller** is the primary HTTP API (Connect RPC) used by the web UI and by AI agents.

## Frontend UI notes

- Prefer semantic button classes from `femtocrank` (via `picocrank`) instead of ad-hoc variants.
- Button styling should generally use only one of: `good`, `neutral`, or `bad`.
- Avoid local button base classes like `btn`; use `femtocrank` semantic classes directly.
- `femtocrank` already defines nearly all CSS needed for normal UI work; prefer those primitives before adding custom styles.

## Discovery endpoints

These are served by the Controller and proxied from the frontend origin:

| Path | Purpose |
|------|---------|
| `/llms.txt` | Human/LLM-readable integration index |
| `/openapi` | OpenAPI 3.1 JSON for the Connect RPC API |
| `/mcp` | MCP Streamable HTTP tools |
| `/api/` | Connect RPC (prefix `/api`) |

## Authentication

The Controller API and MCP currently do not require authentication. When API authentication is enabled, MCP will share the same credentials (`Authorization: Bearer <api-key>`).

Do **not** call `WorkerBootstrap` from agents — it returns AMQP and MySQL credentials for workers. Prefer the MCP tools below or Connect RPCs listed in OpenAPI.

## MCP

- **URL:** `{baseUrl}/mcp`
- **Transport:** Streamable HTTP
- **Library:** [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go)

### Cursor client config (HTTP)

```json
{
  "mcpServers": {
    "icehive": {
      "url": "https://your-host/mcp"
    }
  }
}
```

### Tools

| Tool | Parameters | Behavior |
|------|------------|----------|
| `icehive_init` | — | Connectivity check; returns Controller version |
| `icehive_health` | — | Health status (metadata DB) |
| `icehive_list_services` | — | Service heartbeats (`healthy` / `stale` / `unknown`) |
| `icehive_list_activity` | `limit?` | Recent control-plane activity (heartbeats, enqueues, runs) |
| `icehive_list_config` | — | Config vars (secrets redacted) |
| `icehive_get_config` | `key` | Get one config var |
| `icehive_set_config` | `key`, `value` | Persist one config var |
| `icehive_list_collection_sources` | `collector_type?` | List collection sources |
| `icehive_list_collector_source_schemas` | `collector_type?` | Collector SourceSchema JSON docs |
| `icehive_upsert_collection_source` | `source` (JSON) | Create/update a collection source |
| `icehive_delete_collection_source` | `id` | Delete a collection source |
| `icehive_enqueue_collection_request` | `collection_source_id` **or** `ephemeral_collection` (JSON) | Run collection now |

## Connect RPC (non-MCP)

Agents may call the Connect API under `/api/icehive.v1.ControllerService/…`. See `/openapi` for the full schema. High-value procedures: `Init`, `Health`, `ListServices`, `ListActivity`, `ListConfig`, `GetConfig`, `SetConfig`, `ListCollectionSources`, `ListCollectorSourceSchemas`, `UpsertCollectionSource`, `DeleteCollectionSource`, `EnqueueCollectionRequest`.

## Development notes

- OpenAPI is generated from protobuf via Buf (`protocol/buf.gen.yaml` → `services/controller/gen/openapi.json`). Run `make generate` (or `buf generate` in `protocol/`) after proto changes.
- MCP handlers call the same Controller service methods as Connect RPC (no duplicate HTTP client).
