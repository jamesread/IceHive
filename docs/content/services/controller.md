# Controller

The Controller is the primary API gateway. It exposes a Connect (gRPC-compatible) service defined in `protocol/icehive/v1/controller.proto`.

On startup it prints the resolved configuration file path to stdout, requires **MySQL** (see `config.example.yaml`), applies [golang-migrate](https://github.com/golang-migrate/migrate) migrations, then opens a pool. If the database is unreachable, it logs the error, **sleeps 5 seconds**, and retries until the connection succeeds or the process is interrupted.

## Running

```bash
make -C services/controller run
```

## Configuration

Place **`config.yaml`** (preferred) or `controller.yaml` in the directory passed to `-configdir`. See `services/controller/config.example.yaml` for a template.

| Key | Default | Description |
|-----|---------|-------------|
| `listen` | (flag default `:8080`) | HTTP listen address when set in config |
| `mysql.host` | — | **Required** with other `mysql.*` fields |
| `mysql.port` | `3306` | MySQL port |
| `mysql.user` | — | Database user |
| `mysql.password` | — | Database password |
| `mysql.database` | — | Database name (must already exist) |

### Migrations

Migration files live in `migrations/` next to the config file. The repository includes defaults under `services/controller/migrations/`.

### Health

The `Health` RPC pings MySQL; if the pool is unreachable it returns `UNAVAILABLE`.
