# collector-azure

Pulls data from Azure APIs on a schedule, normalizes it, and publishes to RabbitMQ.

## Run

```bash
make -C services/collector-azure run
```

## Configuration

Optional `collector-azure.yaml` in the directory passed to `-configdir`.

| Key      | Default | Description                          |
|----------|---------|--------------------------------------|
| `listen` | `:8082` | HTTP listen address (metrics/health) |

Shared collector lifecycle: `services/common/pkg/collector`.
