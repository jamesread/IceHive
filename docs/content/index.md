# IceHive

IceHive ingests operational data from outside systems (GitHub repositories, feeds, mailbox APIs, and more), reshapes each payload into shared **entities**, and routes them across **messaging middleware** toward one or more **storage sinks**.

You typically run IceHive entirely from containers: pair the published multi-binary image (**`ICEHIVE_SERVICE`**) with MySQL plus RabbitMQ, then optionally front the bundled web UI behind your favourite ingress tier.

Jump to **[Deploying IceHive](deployment.md)** for Kubernetes-oriented setup, probes, **`icehive_meta`** keys (AMQP URIs, GitHub PAT, sink databases), and the shared **`ICEHIVE_CONTROLLER_URL`** contract.
