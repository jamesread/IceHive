# IceHive

IceHive ingests operational data from outside systems (GitHub repositories, feeds, mailbox APIs, and more), reshapes each payload into shared **entities**, and routes them across **messaging middleware** toward one or more **storage sinks**.

![IceHive web UI home page with live activity feed](assets/marketing/home.png)

You typically run IceHive entirely from containers: pair the published multi-binary image (**`ICEHIVE_SERVICE`**) with MySQL plus RabbitMQ, then optionally front the bundled web UI behind your favourite ingress tier.

Jump to **[Deploying on Kubernetes](deployment/kubernetes.md)** (recommended) or **[Docker](deployment/docker.md)**, and see **[Deploying IceHive](deployment/index.md)** for **`icehive_meta`** keys (AMQP URIs, GitHub PAT, sink databases), and the shared **`ICEHIVE_CONTROLLER_URL`** contract.
