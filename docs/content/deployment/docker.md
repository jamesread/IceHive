# Deploying with Docker

Every IceHive service runs from the same image, `ghcr.io/jamesread/icehive:<version>`. Start one container per service and choose which service each one runs with `ICEHIVE_SERVICE`; see [Deploying IceHive](index.md) for the list of services. Docker is handy for evaluation and single-host setups. For production, [Kubernetes](kubernetes.md) is recommended.

You also need **MySQL** and **RabbitMQ** that every container can reach.

## Controller

```shell
docker run --rm \
  -e ICEHIVE_SERVICE=controller \
  -v /path/to/controller-config:/etc/icehive:ro \
  -p 8080:8080 \
  ghcr.io/jamesread/icehive:<version> \
  -configdir /etc/icehive
```

`/etc/icehive/config.yaml` holds the controller's MySQL connection and `listen` address (see [Controller](../services/controller.md)). Database migrations are bundled in the image at `/opt/ih/migrations`.

## Collectors and persisters

Point every worker at the controller with `ICEHIVE_CONTROLLER_URL`. The workers fetch their AMQP and sink settings from the controller, so their own config is minimal:

```shell
docker run --rm \
  -e ICEHIVE_SERVICE=persister-mysql \
  -e ICEHIVE_CONTROLLER_URL=http://controller:8080 \
  -v /path/to/instance-config:/etc/icehive:ro \
  ghcr.io/jamesread/icehive:<version> \
  -configdir /etc/icehive
```

## Web UI

`ICEHIVE_SERVICE=frontend` serves the UI with NGINX and proxies API calls to the controller set in `ICEHIVE_CONTROLLER_PORT` (`http://host:port`):

```shell
docker run --rm \
  -e ICEHIVE_SERVICE=frontend \
  -e ICEHIVE_CONTROLLER_PORT=http://controller:8080 \
  -p 8081:8080 \
  ghcr.io/jamesread/icehive:<version>
```

The UI listens on port 8080 inside the container; change it with `ICEHIVE_FRONTEND_PORT`.

## Next steps

Bootstrap the AMQP and sink settings in **[`icehive_meta`](index.md#operational-settings-icehive_meta)** so the controller can hand them out to collectors and persisters.
