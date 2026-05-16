# persister-yaml

**Status:** scaffolding. The YAML persister process loads AMQP bootstrap data, opens the shared metrics server, logs readiness, then blocks until terminated—persistent queue consumers and YAML sink writers are **not** wired yet.

Deploy only when experimenting with parity between container packaging and **`persister-mysql`**.

## Container invocation

| Setting | Typical value |
|---------|---------------|
| `ICEHIVE_SERVICE` | `persister-yaml` |
| `ICEHIVE_CONTROLLER_URL` | Controller URL |
| `LOG_LEVEL` | Optional |

Arguments:

```
-configdir /etc/icehive
```

`-listen :8083` default.

## YAML (**`persister-yaml.yaml`**)

| Key | Default |
|-----|---------|
| `listen` | `:8083` |

Future YAML sink knobs (directories, rotations, compaction) belong here once implemented—watch changelog.
