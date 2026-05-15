# collector-testdata

Emits periodic test entities of type `Animal` to RabbitMQ using routing key `collector.entities`.

## Run

```bash
make -C services/collector-testdata run
```

## Configuration

Optional `collector-testdata.yaml` in the directory passed to `-configdir`.

| Key                     | Default | Description                              |
|-------------------------|---------|------------------------------------------|
| `listen`                | `:8085` | HTTP listen address (metrics/health)     |
| `emit_interval_seconds` | `15`    | Emit interval for each test entity batch |

Shared collector lifecycle and HTTP sidecar: `services/common/pkg/collector`.
