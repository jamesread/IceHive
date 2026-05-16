# Controller

The controller is IceHive’s **control plane**. It exposes the Connect surface described in **`icehive.v1.ControllerService`**, stores collection recipes (`collection_sources`), persists operator key/value pairs (`icehive_meta`), aggregates AMQP heartbeat telemetry, publishes on-demand fetch jobs onto RabbitMQ, and answers **`WorkerBootstrap`** so collectors/persisters hydrate secrets at runtime rather than embedding them into each container image.

## Container invocation

Use the unified distribution image (**see [Deploying IceHive](../deployment.md)**):

| Setting | Typical value |
|---------|---------------|
| `ICEHIVE_SERVICE` | `controller` |

Example arguments:

```
-configdir /etc/icehive
```

Optional **`LOG_LEVEL`** env controls verbosity (**`trace` → `panic`**, default **`info`**).

Startup behaviour:

- Locates **`config.yaml`** (preferred) or **`controller.yaml`** under **`-configdir`**.
- Retries **[golang-migrate](https://github.com/golang-migrate/migrate)** + MySQL **`Ping`** until the primary database succeeds (5 second backoff between attempts).
- Opens an AMQP client using **`icehive_meta`** keys (`amqp.*`) with the same retry philosophy as collectors (10 seconds).
- Prints the resolved YAML absolute path plus non-secret MySQL identifiers for easier log tracing.

### Mount layout

Kubernetes should project both:

1. The controller YAML (**`config.yaml`** or **`controller.yaml`**).
2. A sibling **`migrations/`** folder containing packaged SQL revisions.

If either is missing, migrations cannot run and pods stay in crash backoff.

### HTTP surface

Path | Behaviour
-----|-----------
`/` | Lightweight HTML splash referencing RPC + **`/metrics`**
`/metrics` | Prometheus exposition
`/icehive.v1.ControllerService/*` | Connect handlers (Protobuf + JSON codecs)

Expose the listen port behind your mesh/ingress/TLS terminator of choice.

## YAML configuration (**`-configdir`**)

Every key sits in **`config.yaml`** / **`controller.yaml`**:

| Key | Default behaviour | Meaning |
|-----|-------------------|---------|
| `listen` | Omit or set **`auto`** for dynamic selection; **`PORT`** wins when honoured by golure, otherwise controller tries `:8080` then scans `:8000`–`:8999`. **Literal `:8080` also triggers dynamic selection—use `0.0.0.0:8080` (or another explicit host/port) inside Kubernetes.** | Stable HTTP listener binding for RPC + metrics |
| `mysql.host` | _required_ | Controller metadata database host |
| `mysql.port` | `3306` | Controller DB port |
| `mysql.user` | _required_ | Controller DB user |
| `mysql.password` | _required_ | Controller DB password |
| `mysql.database` | _required_ | Must already exist; migrations run here |
| `persister_mysql.*` | Optional | Fallback sink credentials returned to **`persister-mysql`** when **`icehive_meta`** lacks `persister_mysql.*` rows (bootstrap only) |

The sample `worker_bootstrap` stanza sometimes shipped in developer trees is **documentation-only**—runtime distribution still reads **`icehive_meta`**. Populate AMQP + sink keys through SQL or RPC as described in **[Deploying IceHive](../deployment.md#operational-settings-icehive_meta)**.

## Operational metadata vs YAML

Anything workers fetch through **`WorkerBootstrap`** ultimately maps to relational keys (`amqp.url`, `github.token`, `persister_mysql.password`, …). Manage these like any secret:

- Prefer **`SetConfig`** from automation with scoped credentials **or**
- Hydrate **`icehive_meta`** during provisioning (Helm **`post-install` Jobs**, Liquibase callbacks, Terraform `mysql` provider, etc.).

## RPC health semantics

Calling **`Health`** executes a live **`Ping`** against the controller MySQL pool. Failures propagate as **`UNAVAILABLE`**, signalling Kubernetes to defer traffic until retries succeed.

## Related reading

- **AMQP bootstrap keys**: [Deploying IceHive → Operational settings](../deployment.md#amqp-bootstrap-keys-amqp)
- **Shared env/flags**: [Deploying IceHive → Shared environment variables](../deployment.md#shared-environment-variables-and-flags)
