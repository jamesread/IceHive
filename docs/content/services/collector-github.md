# collector-github

Pulls data from the GitHub API on a schedule, normalizes it, and publishes to RabbitMQ.

## Run

```bash
make -C services/collector-github run
```

## Configuration

Optional `collector-github.yaml` in the directory passed to `-configdir`.

| Key      | Default | Description                          |
|----------|---------|--------------------------------------|
| `listen` | `:8081` | HTTP listen address (metrics/health) |

Shared collector lifecycle and HTTP sidecar: `services/common/pkg/collector`.
