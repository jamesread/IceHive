# Architecture

IceHive divides responsibilities between a centralized **controller**, interchangeable **collector** workers, interchangeable **persister** workers, and an optional browser **frontend**.

```
  External SaaS/APIs (GitHub, Azure, RSS/JMAP feeds, ...)
            |
       collector workloads  -->  RabbitMQ (topic exchanges) -->  persister workloads --> sink databases / files
                 |                                                          ^
                 |                                                          |
                 +------------------> Controller <---------------------------+
                                             |
                                          Frontend UI
```

- **Controllers** authenticate scheduled work, expose an HTTP **Connect RPC** façade, persist collection recipes (`collection_sources`) and operator settings (`icehive_meta`), and issue **WorkerBootstrap** responses so every worker receives AMQP plus sink credentials without baking secrets into each container.
- **Collectors** poll or stream remote systems, publish normalized JSON entities with routing key **`collector.entities`**, and report run outcomes back to the controller.
- **Persisters** bind durable queues to the same routing key and land rows in MySQL (current production path) or future filesystem sinks.
- **Frontend** is a static web bundle that points at the controller base URL (build-time or user-selected) to inspect service heartbeats and configuration.

For concrete deployment patterns (Service layout, configuration mounts, health checks, bootstrapping **`icehive_meta`**), read **[Deploying IceHive](deployment.md)**.
