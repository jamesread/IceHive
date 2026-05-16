# persister-mysql

The production sink worker binds queue **`ih_persister-mysql-entities`** (logical name `persister-mysql-entities` with IceHive queue prefix) to routing key **`collector.entities`**, decodes JSON entities, auto-creates/alter tables per **`entity_type`**, and **`INSERT … ON DUPLICATE KEY UPDATE`** keyed by **`source_hash_value`**.

## Container invocation

| Setting | Typical value |
|---------|---------------|
| `ICEHIVE_SERVICE` | `persister-mysql` |
| `ICEHIVE_CONTROLLER_URL` | Controller URL (**must respond to `WorkerBootstrap`**) |
| `LOG_LEVEL` | Optional |

Arguments:

```
-configdir /etc/icehive
```

Default **`/healthz`** bind **`:8084`**.

Persisters authenticate to **sink MySQL** solely through bootstrap payloads—never embed passwords in collector YAML.

## Bootstrap credentials pathway

During **`WorkerBootstrap`**, **`persister-mysql`** requests MySQL knobs when `worker_kind == persister`. The controller resolves in order:

1. **`icehive_meta`** keys prefixed **`persister_mysql.`** (recommended production path—see **[Deploying IceHive](../deployment.md#sink-database-keys-persister_mysql)**).
2. YAML block **`persister_mysql`** shipped with controller config if relational storage lacks those rows (temporary bootstrap shortcut).

Either way manifests as host/port/user/password/database fields inside the Connect response.

## Optional YAML (**`persister-mysql.yaml`**)

| Key | Default | Purpose |
|-----|---------|---------|
| `listen` | `:8084` | Metrics/health bind |

No database DSN fields appear here by design.

## Storage model snapshot

Each entity type becomes its own table (pluralized snake case) with metadata columns (`source_unique_id`, `observed_unix_ms`, `recollect_spec`, …) plus dynamic columns inferred from the JSON `structure` map. Schema evolution issues additive **`ALTER TABLE`** statements when new fields appear.

## Operational guidance

- Keep sink credentials distinct from controller metadata DB roles.
- Monitor queue depth on **`ih_persister-mysql-entities`**; sustained growth indicates DB pressure or poison messages.
- Treat horizontal scaling cautiously—each replica competes as a RabbitMQ consumer (which is generally fine), but concurrent schema alterations on the sink may race when new attributes appear simultaneously.
