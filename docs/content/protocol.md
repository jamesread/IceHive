# Wire protocol and schema

Public APIs are defined with **Protocol Buffers (proto3)** and delivered over **Connect**, so every RPC is available as **gRPC** *or* **JSON over HTTP/1.1** on the controller’s listen port (paths look like `/icehive.v1.ControllerService/...`).

## What operators need to know

- Controllers load TLS credentials the same way as any other HTTP server you terminate in front of them (ingress, service mesh, or sidecar). Workers only need a routable **base URL** passed through **`ICEHIVE_CONTROLLER_URL`**.
- Entity payloads on the bus are JSON documents that include `collectormetadata` plus `structure`/`values` maps; persist MySQL tables are generated per `entity_type`.
- Control-plane metadata (AMQP URL, GitHub token, sink DB credentials) lives in **`icehive_meta`** key/value rows accessed through **`GetConfig` / `SetConfig` / `ListConfig`**.

## Default AMQP routing vocabulary

| Topic / prefix | Usage |
|----------------|-------|
| `control.events` | Cross-service coordination envelopes |
| `control.heartbeats` | Liveness signals recorded in the controller DB |
| `collector.entities` | Normalized entity stream consumed by persist workers |
| `collector.collection_request.<collector-type>` | On-demand collection fan-out |
| `collector.source_schema.<collector-type>` | Published JSON schema hints |
| `ex_icehive` | Durable topic exchange name unless overridden through meta |

If you extend IceHive, keep new routing keys namespaced under these prefixes to avoid collisions with operator-defined broker policies.

## UI schema integration

The browser bundle ships with generated TypeScript stubs that mirror the same `.proto` sources. When you rebuild the UI for production, point **`VITE_CONTROLLER_BASE_URL`** at the controller endpoint your users will hit (see **[Frontend](frontend.md)**).
