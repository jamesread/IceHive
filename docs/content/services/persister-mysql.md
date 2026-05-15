# persister-mysql

Consumes normalized messages from RabbitMQ and persists them with a MySQL-backed `persist.Store`.

## Run

```bash
make -C services/persister-mysql run
```

## Configuration

Optional `persister-mysql.yaml` in the directory passed to `-configdir`.

| Key      | Default | Description                          |
|----------|---------|--------------------------------------|
| `listen` | `:8084` | HTTP listen address (metrics/health) |

Shared persister lifecycle: `services/common/pkg/persist`.
