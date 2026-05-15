# persister-yaml

Consumes normalized messages from RabbitMQ and writes them using a YAML filesystem (or similar) sink via `persist.Store`.

## Run

```bash
make -C services/persister-yaml run
```

## Configuration

Optional `persister-yaml.yaml` in the directory passed to `-configdir`.

| Key      | Default | Description                          |
|----------|---------|--------------------------------------|
| `listen` | `:8083` | HTTP listen address (metrics/health) |

Shared persister lifecycle and `persist.Store`: `services/common/pkg/persist`.
