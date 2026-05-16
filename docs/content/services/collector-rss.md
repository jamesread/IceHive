# collector-rss

**RSS ingests Atom/RSS URLs defined as collection sources.** The collector downloads feeds, clamps item counts per feed, honours HTTP timeouts/custom user agents configured in YAML, and publishes normalized entities downstream.

## Container invocation

| Setting | Value |
|---------|-------|
| `ICEHIVE_SERVICE` | `collector-rss` |
| `ICEHIVE_CONTROLLER_URL` | Controller base URL |

```
-configdir /etc/icehive
```

`-listen :8087` default.

## Optional YAML (`collector-rss.yaml`)

| Key | Default | Purpose |
|-----|---------|---------|
| `listen` | `:8087` | Metrics + health listener |
| `poll_interval_seconds` | `60` | Delay between scans of relational collection recipes |
| `fetch_timeout_seconds` | `90` | HTTP client timeout (**seconds**); must parse as a positive integer to override the **`90`** second built-in fallback |
| `user_agent` | _(see env fallback)_ | When non-empty in YAML it becomes the `User-Agent`; otherwise **`ICEHIVE_RSS_USER_AGENT`** supplies the fallback before **`gofeed`** defaults kick in |
| `items_max_per_feed` | `25` | Max items persisted per polled feed |

Feeds themselves live in relational **`collection_sources`** rows whose **`collector_type`** is **`collector-rss`** (YAML adjusts transport ergonomics only).

### Environment overrides

| Variable | Behaviour |
|-----------|-----------|
| `ICEHIVE_RSS_USER_AGENT` | Used whenever YAML omits **`user_agent`** |

## Routing keys

Schemas publish under **`collector.source_schema.collector-rss`**; enqueue topics use **`collector.collection_request.collector-rss`** suffixes.
