# collector-testdata

**Testdata** emits deterministic **`Animal`** JSON entities roughly every **`emit_interval_seconds`** so QA clusters can rehearse ingestion without touching SaaS integrations.

Perfect for validating RabbitMQ quotas, **`persister-mysql`** DDL generation, alerting, dashboards, …

## Container invocation

| Setting | Typical value |
|---------|---------------|
| `ICEHIVE_SERVICE` | `collector-testdata` |
| `ICEHIVE_CONTROLLER_URL` | Controller base URL |
| `LOG_LEVEL` | Optional |

Arguments:

```
-configdir /etc/icehive
```

Default **`/healthz`** port comes from `:8085` unless replaced.

## Optional YAML (**`collector-testdata.yaml`**)

| Key | Default | Notes |
|-----|---------|-------|
| `listen` | `:8085` | Overrides metrics/health bind |
| `emit_interval_seconds` | `15` | Minimum positive integer seconds between replay batches |

Each tick publishes three fixtures with routing key **`collector.entities`**.
