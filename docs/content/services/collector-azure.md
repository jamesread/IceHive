# collector-azure

The Azure-facing collector publishes source-schema metadata onto RabbitMQ and currently keeps the workload alive while deeper Azure ingestion logic finishes wiring upstream.

Operate it whenever you intentionally want placeholders for **`collector-azure`** **`collection_sources`**, broker bindings, or future Graph/ARM collectors.

## Container invocation

| Setting | Typical value |
|---------|---------------|
| `ICEHIVE_SERVICE` | `collector-azure` |
| `ICEHIVE_CONTROLLER_URL` | Controller base URL |
| `LOG_LEVEL` | Optional |

Arguments:

```
-configdir /etc/icehive
```

Default HTTP sidecar **`-listen :8082`** unless YAML overrides.

## Optional YAML (**`collector-azure.yaml`**)

| Key | Default | Purpose |
|-----|---------|---------|
| `listen` | `:8082` | Metrics/health listener |

No Azure credentials are read from disk yet; expect future keys to land here or in **`icehive_meta`** once the integration hardens. Track release notes when those appear.

## Observability

Same pattern as other collectors: **`GET /healthz`**, **`/metrics`**.
