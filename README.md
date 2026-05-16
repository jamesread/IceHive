<div align="center">
  <img alt="IceHive logo" src="logo.svg" width="128" />
  <h1>IceHive</h1>

  IceHive is a set of microservices that **collect** data from various APIs, **normalize** it into a standard format, and **store** it in a database.

[![Maturity](https://img.shields.io/badge/maturity-Development-yellow)](#none)
[![Go Report Card](https://goreportcard.com/badge/github.com/icehive/icehive)](https://goreportcard.com/report/github.com/icehive/icehive)

</div>

## What it does

- **Controller** exposes a gRPC API (Connect) for clients to query and request data.
- **Collector binaries** (for example `collector-github`, `collector-azure`, `collector-testdata`) run on a schedule, pull data from a specific vendor API (or emit deterministic test fixtures), normalize it, and publish to RabbitMQ. Shared collector runtime code lives in **common** (`pkg/collector`).
- **Persister binaries** (for example `persister-yaml`, `persister-mysql`) consume normalized messages from the queue and write them using a sink-specific implementation of `persist.Store` in **common** (`pkg/persist`).
- **Common** holds shared library code used by all services (configuration loading, AMQP control helpers, HTTP auth shim wrapper, etc.).

The **frontend** is a web application that talks to the Controller for viewing and requesting data.

## Getting started

You need Go, Node.js, Make, and [Buf](https://buf.build/docs/installation/) for protocol buffers. Clone the repository, generate code, build, and run the component you care about:

```bash
make
```

See `Makefile` targets for build, test, lint, and running individual services. Developer-oriented documentation lives under `docs/` (MkDocs).
