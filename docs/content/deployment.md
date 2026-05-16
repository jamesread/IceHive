# Deploying IceHive

IceHive ships as a **single Linux container image** that bundles every backend binary plus a small entrypoint. You pick which workload runs via **`ICEHIVE_SERVICE`**, which aligns with **Kubernetes** (one Deployment per service, identical image digest).

Official release images carry the registry and tags listed on GitHub Releases (commonly **`ghcr.io/jamesread/icehive:<version>`**).

## How the container starts

Inside the image the entrypoint runs:

```shell
exec "/usr/local/bin/${ICEHIVE_SERVICE}" "$@"
```

Anything you append after the image name (`args` in Kubernetes, trailing arguments after `docker run`) is passed straight through (for example **`-configdir /etc/icehive`**).

## Choosing `ICEHIVE_SERVICE`

| `ICEHIVE_SERVICE` value | Role |
|------------------------|------|
| `controller` | Connect (gRPC-compatible) HTTP API, MySQL for platform metadata, distributes worker bootstrap data |
| `collector-github` | Pulls configured GitHub repositories on a cron schedule |
| `collector-azure` | Azure integrations (minimal behaviour today—see service page) |
| `collector-jmap` | JMAP mailbox ingestion |
| `collector-rss` | RSS/Atom ingestion |
| `collector-testdata` | Emits fixed test entities onto the bus |
| `persister-mysql` | Dynamically tails entity messages into a sink MySQL database |
| `persister-yaml` | Placeholder YAML sink (consumes bootstrap only—see service page) |

### Example Docker

```shell
docker run --rm \
  -e ICEHIVE_SERVICE=persister-mysql \
  -e ICEHIVE_CONTROLLER_URL=http://controller:8080 \
  -v /path/to/instance-config:/etc/icehive:ro \
  ghcr.io/jamesread/icehive:v0.1.2 \
  -configdir /etc/icehive
```

## Typical Kubernetes footprint

Provision these shared dependencies:

1. **MySQL cluster A (“controller”)** holding IceHive operational tables (`collection_sources`, `icehive_meta`, heartbeat tracking, migrations applied by controller).
2. **MySQL cluster B (“sink”)** *optional but recommended*: dedicated database reachable only by **`persister-mysql`** credentials you publish through the Controller.
3. **RabbitMQ** (compatible AMQP URI) reachable from every Pod.

Recommended workload split:

| Workload category | Scheduling notes |
|------------------|------------------|
| `controller` | Mount config + migrations; expose Service/Ingress suitable for HTTPS termination if externally facing |
| collectors / persisters | Set `ICEHIVE_CONTROLLER_URL` to your controller Service Cluster DNS |

### Observability probes

Collectors and persisters expose Prometheus metrics on **`/metrics`** plus **`GET /healthz`** (**`200`** body `ok`). Use HTTP probes targeting **`/healthz`**.

The **controller** exposes **`/metrics`** and the Connect endpoints but **not** **`/healthz`**. Prefer a **`tcpSocket`** readiness probe matching the RPC port until you add an HTTP-compatible health shim.

Pin stable HTTP binds in YAML. Collectors/persisters can usually set **`listen: ":8080"`**, but **`controller`** treats **`":8080"` specially (it participates in dynamic port allocation). Use **`listen: "0.0.0.0:8080"`**—or another explicit **`host:port`**—inside Kubernetes so Deployments/Services/targetPorts stay aligned.

### Configuration volumes

Treat `/etc/icehive` as a standard mount inside every Pod (`emptyDir`, **ConfigMap** + **Secret** projections, CSI, etc.). Start every process with **`args: ["-configdir", "/etc/icehive"]`** unless you reorganize layouts.

Secrets (password fields) normally live in Kubernetes **Secrets** layered under the same path.

---

## Operational settings (`icehive_meta`)

The Controller distributes AMQP URIs/exchange knobs and **`persister-mysql`** sink credentials to collectors and persisters via **`WorkerBootstrap`**. Values come from relational rows keyed by **dot-separated names** stored in **`icehive_meta`** (readable through the **`ListConfig`** / **`GetConfig`** / **`SetConfig`** RPCs).

Bootstrap your cluster by inserting/updating rows through migrations, **`kubectl exec`**, DBA tooling, automation that calls **`SetConfig`**, or Helm hooks.

### AMQP bootstrap keys (`amqp.%`)

Either supply a composite URI (`amqp.url`) **or** individual fields that the Controller assembles internally before connecting.

| Meta key | Required / optional | Meaning |
|-----------|---------------------|---------|
| `amqp.url` | Prefer | Full AMQP URI, e.g. `amqp://user:pass@rabbitmq.icehive.svc:5672/vhost` |
| `amqp.host` | When `url` absent | Broker hostname/IP |
| `amqp.port` | Optional | Broker port (**`5672`** when blank) |
| `amqp.user` | Optional | Username inside URI assembly |
| `amqp.password` | Optional | Password inside URI assembly |
| `amqp.vhost` | Optional | `/` virtual host semantics |
| `amqp.exchange` | Optional | Topic exchange (**`ex_icehive`** when omitted) |
| `amqp.routing_key_control_events` | Optional | Routing key (**`control.events`** when omitted) |

### Sink database keys (`persister_mysql.%`)

| Meta key | Required | Meaning |
|-----------|----------|---------|
| `persister_mysql.host` | Yes | Reachable hostname from persister Pods |
| `persister_mysql.port` | Implicit | **`3306`** when unset |
| `persister_mysql.user` | Yes | Sink DB user |
| `persister_mysql.password` | Yes | Sink DB password |
| `persister_mysql.database` | Yes | Sink logical database |

If **`persister-mysql`** cannot read these rows, the YAML block **`persister_mysql`** on the **controller Pod** temporarily supplies sink settings as a bootstrap fallback—but normal operations should hydrate **`icehive_meta`**.

---

## Collector-specific meta keys (`github.token`)

GitHub ingest reads the **`GetConfig("github.token")`** value to authenticate API calls. Store that token in **`icehive_meta`** (or set it through RPC). Treat it as a secret in Kubernetes.

---

## Shared environment variables and flags

| Name | Applies to | Description |
|------|-------------|-------------|
| `ICEHIVE_SERVICE` | Container entrypoint | Selects the binary to launch |
| `LOG_LEVEL` | All backend binaries | `trace` · `debug` · `info` · `warn` · `error` · `fatal` · `panic` (invalid → `info`) |
| `ICEHIVE_CONTROLLER_URL` | Collectors + persisters | Connect base URL (`http`/`https`, no trailing slash). Falls back to `http://localhost:8080` when unset—almost never correct in-cluster |
| CLI `-configdir` | All services | Directory containing YAML (and controller `migrations/`) |
| CLI `-controller` | Collectors + persisters | Overrides `ICEHIVE_CONTROLLER_URL` when non-empty |
| CLI `-listen` | All services | HTTP bind address; defaults per binary but **YAML `listen` wins** when present |

Service-specific YAML keys are documented on each service page under **Configuration**.
