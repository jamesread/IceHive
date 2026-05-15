IceHive is a set of microservices that work together to collect data from various APIs, normalize it into a standard format, and store it in a database.

Services:

- Controller (gRPC API)
- Collector binaries (for example collector-github, collector-azure, collector-testdata) — periodically collect from a specific vendor API (or emit deterministic test entities) and publish normalized data to a message queue (RabbitMQ). Shared collector runtime lives in the common library (`pkg/collector`).
- Persister binaries (for example persister-yaml, persister-mysql) — consume normalized data from the message queue and persist via a sink-specific `persist.Store` in the common library (`pkg/persist`).
- Common library (shared code for all services)

Clients:

- Frontend (Vite web application) - allows for viewing and requesting data from the Controller service

Specifications:

- [Collector Service Specification](collector-service.md) - defines collector responsibilities, normalized entity format, source-to-entity mapping rules, and persistor delivery contract.
