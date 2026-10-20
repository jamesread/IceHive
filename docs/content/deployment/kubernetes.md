# Deploying on Kubernetes

Kubernetes is the recommended way to run IceHive. Every service runs as its own Deployment from the **same image** (`ghcr.io/jamesread/icehive:<version>`); `ICEHIVE_SERVICE` selects the workload. See [Deploying IceHive](index.md) for the list of services and how the container starts.

## Example manifest

A minimal multi-document manifest that runs the **controller** (ConfigMap-mounted `config.yaml`, SQL migrations baked into the container at `/opt/ih/migrations`, ClusterIP Service, `tcpSocket` probes) lives at [`examples/kubernetes-icehive.yaml`](../examples/kubernetes-icehive.yaml). Edit image tag, hostnames, and passwords, deploy MySQL and RabbitMQ (or point at existing brokers / databases), then `kubectl apply -f` the file. Add collectors and persisters as separate Deployments using the **same** image and `ICEHIVE_CONTROLLER_URL=http://icehive-controller.icehive.svc.cluster.local:8080` as described in [Shared environment variables](index.md#shared-environment-variables-and-flags).

Provision these shared dependencies:

1. **MySQL cluster A (“controller”)** holding IceHive operational tables (`collection_sources`, `icehive_meta`, heartbeat tracking, migrations applied by controller).
2. **MySQL cluster B (“sink”)** *optional but recommended*: dedicated database reachable only by **`persister-mysql`** credentials you publish through the Controller.
3. **RabbitMQ** (compatible AMQP URI) reachable from every Pod.

Recommended workload split:

| Workload category | Scheduling notes |
|------------------|------------------|
| `controller` | Mount **config only** (migrations ship in the image under `/opt/ih/migrations`); expose Service/Ingress suitable for HTTPS termination if externally facing |
| collectors / persisters | Set `ICEHIVE_CONTROLLER_URL` to your controller Service Cluster DNS |

## Observability probes

Collectors and persisters expose Prometheus metrics on **`/metrics`** plus **`GET /healthz`** (**`200`** body `ok`). Use HTTP probes targeting **`/healthz`**.

The **controller** exposes **`/metrics`** and the Connect endpoints but **not** **`/healthz`**. Prefer a **`tcpSocket`** readiness probe matching the RPC port until you add an HTTP-compatible health shim.

Pin stable HTTP binds in YAML. Collectors/persisters can usually set **`listen: ":8080"`**, but **`controller`** treats **`":8080"` specially (it participates in dynamic port allocation). Use **`listen: "0.0.0.0:8080"`**—or another explicit **`host:port`**—inside Kubernetes so Deployments/Services/targetPorts stay aligned.

## Configuration volumes

Treat `/etc/icehive` as a standard mount inside every Pod (`emptyDir`, **ConfigMap** + **Secret** projections, CSI, etc.). Start every process with **`args: ["-configdir", "/etc/icehive"]`** unless you reorganize layouts.

Secrets (password fields) normally live in Kubernetes **Secrets** layered under the same path.

## Next steps

Bootstrap the AMQP and sink settings in **[`icehive_meta`](index.md#operational-settings-icehive_meta)** so the controller can hand them out to collectors and persisters.
