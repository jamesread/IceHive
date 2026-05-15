# Collector Service Specification

## Purpose and Scope

Collectors gather data from external systems (for example public APIs), normalize that data into a shared entity model, and publish normalized entities for persistence.

This specification defines:

- how collectors represent normalized entities
- how source objects are mapped into entities
- what contract persistors can rely on
- error handling and observability requirements

This specification does **not** define source-specific storage schemas inside collectors. Collectors only produce normalized entities; persistors own sink-specific storage layout.

## Runtime Context

Collectors run as service-specific binaries (for example `collector-github`, `collector-azure`) on top of the shared runtime in `services/common/pkg/collector/run.go`.

At runtime, a collector MUST:

1. bootstrap from the controller (`WorkerBootstrap`) to fetch AMQP settings
2. connect to AMQP and keep heartbeats publishing through shared `amqpctl` logic
3. run source-specific collection logic in its `Work` function
4. emit normalized entity messages to AMQP for persistors

Collectors MUST publish entity messages with routing key `collector.entities`.

## Canonical Entity Model

Collectors MUST normalize source records into a generic entity message.

### Entity Message

Each emitted entity MUST include these root-level attributes:

- `type` (string): message kind discriminator (MUST be `Entity`)
- `schema_version` (string): schema contract version (MUST be `v1`)
- `collectormetadata` (object): collector metadata for this entity
- `structure` (object map): definitions for all fields contained in `values`, keyed by field name
- `values` (object): normalized entity field values

`collectormetadata` MUST include:

- `entity_type` (string): canonical logical type, PascalCase (example: `GitRepo`)
- `source_system` (string): origin system identifier (example: `github`)
- `source_collector_type` (string): collector identifier (example: `collector-github`)
- `source_unique_id` (string): stable source-native identifier (example: repository node id)
- `source_hash` (object): deterministic source identity hash with:
  - `hash_value` (string): hash of `source_unique_id` + `:` + `source_collector_type`
  - `hash_type` (string): hash algorithm identifier (default and required baseline value: `sha256`)
- `observed_unix_ms` (int64): when this snapshot was observed by the collector

`collectormetadata` MAY include:

- `recollect_spec` (string or JSON `null`): a **hint** for a collection `source_spec` (same opaque string the collector would accept on a dedicated collection source) that would allow **re-fetching this specific entity** or the smallest scope that contains it. The hint MAY be **more specific** than the collection source that produced the entity (for example the active source might be `org.repos:jamesread` while a `GitRepo` row for repository `faridoon` could set `recollect_spec` to `repo:jamesread/faridoon`). When a collector cannot express a narrower or actionable spec for a single entity, it MUST set `recollect_spec` to JSON `null` (or omit the key; persistors and UIs MUST treat omitted and `null` equivalently as “no hint”).
- when `recollect_spec` is a string, it MUST be UTF-8 and normalized to Unicode NFC, and it SHOULD use the same syntax conventions as that collector’s documented `source_spec` (including optional `+modifier` tokens where applicable).

### Structure and Value Rules

- `structure` MUST be an object map where each key is a field name and each value is that field's descriptor
- each `structure` descriptor MUST include:
  - `type` (string): scalar data type (`string`, `int64`, `float64`, `bool`)
- each `structure` descriptor SHOULD include `length` when relevant (for example max string length constraints)
- `values` keys MUST use stable snake_case naming where practical (`name`, `stars`, `is_private`, `default_branch`)
- `values` MUST be scalars only (`string`, `int64`, `float64`, `bool`)
- `values` MUST be deterministic for identical source state
- missing source fields SHOULD be omitted rather than emitted as empty strings
- nested structures from source systems MUST NOT be emitted in `values`
- nested source data MUST be emitted as additional entities with their own `collectormetadata`, `structure`, and `values`

### Validation Contract

Entity messages MUST satisfy all validation rules below:

- root object MUST contain at least `type`, `schema_version`, `collectormetadata`, `structure`, and `values`
- root object MAY include additional fields for forward-compatible extensions
- `type` MUST equal `Entity`
- `schema_version` MUST equal `v1`
- `collectormetadata` MUST contain all required keys:
  - `entity_type`, `source_system`, `source_collector_type`, `source_unique_id`, `source_hash`, `observed_unix_ms`
- `collectormetadata.recollect_spec`, when present, MUST be either a JSON string or JSON `null` (not a number, object, or array)
- `collectormetadata.source_hash` MUST be an object containing required keys `hash_value` and `hash_type`
- `collectormetadata.source_hash.hash_type` MUST equal `sha256`
- `collectormetadata.source_hash.hash_value` MUST be computed from `source_unique_id` + `:` + `source_collector_type` using `sha256`
- all string values in entity messages MUST be UTF-8 encoded and normalized to Unicode NFC
- `collectormetadata.source_hash.hash_value` MUST be emitted as lowercase hexadecimal
- `structure` MUST be an object (not an array)
- each key in `structure` MUST be a non-empty string and MUST also exist in `values`
- each value in `structure` MUST be an object with required key `type` (string)
- allowed `type` values for baseline compatibility are:
  - `string`, `int64`, `float64`, `bool`
- if a descriptor contains `length`, it MUST be a positive integer
- `values` MUST be an object (not an array)
- each key in `values` MUST also exist in `structure`
- for `structure.<field>.type = int64`, corresponding `values.<field>` MUST be an integer
- for `structure.<field>.type = float64`, corresponding `values.<field>` MUST be numeric
- for `structure.<field>.type = bool`, corresponding `values.<field>` MUST be boolean
- for `structure.<field>.type = string`, corresponding `values.<field>` MUST be string
- `values` MUST NOT contain objects or arrays

## Mapping Rules

Collectors MUST implement deterministic transformations:

- same source object state -> same `entity_type` and normalized attribute keys
- source identifiers MUST map to stable `source_unique_id`
- `source_hash.hash_value` MUST be deterministically derived from `source_unique_id` and `source_collector_type`
- hashing inputs MUST be the UTF-8 NFC-normalized string values of `source_unique_id` and `source_collector_type`
- collector version changes MUST NOT silently rename existing keys
- when a collector emits `recollect_spec` as a string, it SHOULD be stable for the same underlying source record (same entity identity implies the same hint string)

Collectors SHOULD keep mapping logic isolated from fetch logic so mapping can be tested independently.

## GitHub Example Mapping

### Source Object (example)

```json
{
  "id": 123456,
  "node_id": "R_kgDOGH123",
  "name": "icehive",
  "full_name": "icehive/icehive",
  "stargazers_count": 300,
  "forks_count": 42,
  "private": false,
  "default_branch": "main",
  "html_url": "https://github.com/icehive/icehive"
}
```

### Normalized Entity (example)

```json
{
  "type": "Entity",
  "schema_version": "v1",
  "collectormetadata": {
    "entity_type": "GitRepo",
    "source_system": "github",
    "source_collector_type": "collector-github",
    "source_unique_id": "R_kgDOGH123",
    "source_hash": {
      "hash_type": "sha256",
      "hash_value": "fefde5eb052842db5f1ddc65f4bdcb5d685b2f6de5ecf5af43a4d1f0190709d8"
    },
    "observed_unix_ms": 1777545000000,
    "recollect_spec": "repo:icehive/icehive"
  },
  "structure": {
    "name": { "type": "string", "length": 255 },
    "full_name": { "type": "string", "length": 255 },
    "stars": { "type": "int64" },
    "forks": { "type": "int64" },
    "is_private": { "type": "bool" },
    "default_branch": { "type": "string", "length": 255 },
    "url": { "type": "string", "length": 2048 }
  },
  "values": {
    "name": "icehive",
    "full_name": "icehive/icehive",
    "stars": 300,
    "forks": 42,
    "is_private": false,
    "default_branch": "main",
    "url": "https://github.com/icehive/icehive"
  }
}
```

If the same repository were collected under a broader source (for example `org.repos:icehive`), `recollect_spec` would still narrow to `repo:icehive/icehive` so a downstream tool can offer “re-sync this repo” using a single-repo `source_spec`.

## Delivery Contract to Persistors

Persistors consume normalized entities and derive sink artifacts from `entity_type`.

Persistors MUST consume entity messages published on routing key `collector.entities`.

### Entity Type to Sink Name

- `entity_type` MUST be PascalCase.
- Persistors MUST derive canonical sink names by:
  - lowercasing
  - pluralizing with trailing `s` (initial baseline rule)

Examples:

- `GitRepo` -> `gitrepos` (MySQL table), `gitrepos.yaml` (YAML sink)
- `CloudVm` -> `cloudvms`, `cloudvms.yaml`

If pluralization behavior is enhanced later, persistors MUST retain backward compatibility for previously materialized names.

### Idempotency and Deduplication Expectations

- collector output MAY contain repeated snapshots for the same `source_unique_id`
- persistors SHOULD upsert by (`entity_type`, `source_hash.hash_value`) where sink supports it
- persistors MUST tolerate duplicate messages without creating unbounded duplicate records

## Error Handling and Retries

Collectors MUST:

- retry transient source API errors with bounded backoff
- log and skip malformed source records rather than terminating the process
- continue emitting valid entities even when some records fail normalization
- rely on shared runtime retry behavior for controller bootstrap and AMQP connect

Collectors SHOULD expose per-run or per-poll counts for fetched, normalized, emitted, and failed records.

## Observability

Collectors MUST emit logs for:

- poll start/finish per source
- normalization failures (with enough context to debug)
- emit success/failure summaries
- AMQP connection status and heartbeat send outcomes (inherited from shared common AMQP package)

Collectors SHOULD provide metrics for:

- fetch latency
- normalize error count
- emitted entity count per `entity_type`

## Versioning and Compatibility

Entity model evolution rules:

- adding new `values` fields (and corresponding `structure` map entries) is backward-compatible
- adding optional `collectormetadata` keys (such as `recollect_spec`) is backward-compatible
- removing or renaming `values` fields is breaking and MUST be versioned/migrated deliberately
- changing `entity_type` for existing data is breaking and MUST include persistor migration guidance

Where a source API changes semantics, collector owners MUST document mapping changes before rollout.

## Acceptance Criteria

A collector implementation is compliant when:

- it bootstraps and connects through shared runtime contracts
- it emits entities matching the canonical message format (`type`, `schema_version`, `collectormetadata`, `structure`, `values`)
- it provides deterministic mapping for supported source objects
- `entity_type` naming is compatible with persistor sink derivation rules
- duplicate snapshots do not break persistence expectations
- required logs/metrics exist for fetch, normalize, and emit paths
