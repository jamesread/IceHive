# Architecture

IceHive is composed of a controller, pluggable collector and persister binaries, plus a web frontend.

## Data flow

```
  External APIs
       |
   collector-*  -->  RabbitMQ  -->  persister-*  -->  Sink (DB / files / …)
                                                      |
                   Controller  <-----------------------+
                       |
                   Frontend
```

- **Collector binaries** (for example `collector-github`, `collector-azure`, `collector-testdata`) periodically pull data from a specific vendor API (or emit deterministic fixture entities), normalize it, and publish messages to RabbitMQ. Shared runtime behaviour lives in `services/common/pkg/collector`.
- **Persister binaries** (for example `persister-yaml`, `persister-mysql`) consume those messages and write records using a sink-specific `persist.Store` implementation. Shared runtime behaviour lives in `services/common/pkg/persist`.
- **Controller** exposes a Connect (gRPC) API that clients use to query stored data and trigger on-demand collection.
- **Frontend** is a Vue/Vite web application that communicates with the Controller.
- **Common** is a shared Go library (`config`, `collector`, `persist`, AMQP helpers, etc.) used by all backend binaries.
