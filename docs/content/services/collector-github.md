# collector-github

The GitHub collector lists enabled **`collection_sources`** whose `collector_type` matches **`collector-github`**, honours each source’s **`cron_line`**, fetches repositories/Dependabot alerts/PR summaries from the upstream REST API, and publishes JSON entities routing them under **`collector.entities`**.

Each container/pod must reach **both**:

1. **`ICEHIVE_CONTROLLER_URL`** → Connect API for **`ListCollectionSources`**, **`ReportCollectionSourceRun`**, **`GetConfig`**, AMQP heartbeat plumbing, …
2. Public or private **GitHub** endpoints (internet egress unless you tunnel).

## Container invocation

| Setting | Typical value |
|---------|---------------|
| `ICEHIVE_SERVICE` | `collector-github` |
| `ICEHIVE_CONTROLLER_URL` | Operator controller Service URL (**include scheme, no trailing slash**) |
| `LOG_LEVEL` | Optional (`info` default) |

Arguments:

```
-configdir /etc/icehive
```

You can also pass **`-listen :8081`** on the command line; if **`listen`** appears in YAML, the YAML value wins.

## Required controller metadata

| `icehive_meta` key | Purpose |
|--------------------|---------|
| `github.token` | Personal access token or GitHub App installation token with scopes appropriate for your configured repositories (used via **`GetConfig`**) |

Missing or blank tokens degrade to anonymous GitHub REST limits—avoid in production.

## Optional YAML (**`collector-github.yaml`** inside **`-configdir`**)

Loaded only when present; filenames are lowercase with service suffix.

| YAML key | Default | Purpose |
|---------|---------|---------|
| `listen` | `:8081` | HTTP bind for **`/metrics`** & **`/healthz`** when set (overrides the **`-listen`** flag) |
| `poll_interval_seconds` | `60` | How often the collector re-queries the Controller for collection sources (seconds) |

Anything else belongs in relational **`collection_sources.source_spec`** (repository scopes, Dependabot toggle, PR snapshot toggle, …) rather than static YAML because operators iterate those rows through RPC/UI.

### Health endpoints

`/healthz` for probes, `/metrics` for Prometheus scraping via your Service monitors.

### Related routing keys

- Entity publish → **`collector.entities`**
- On-demand jobs → **`collector.collection_request.collector-github`**
- Published schema descriptors → **`collector.source_schema.collector-github`**

## Related docs

[Deploying IceHive → Collector-specific meta](../deployment.md#collector-specific-meta-keys-githubtoken)
