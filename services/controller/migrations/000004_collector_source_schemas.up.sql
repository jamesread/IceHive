CREATE TABLE IF NOT EXISTS icehive_collector_source_schemas (
    collector_type VARCHAR(128) NOT NULL PRIMARY KEY,
    schema_version VARCHAR(32) NOT NULL,
    body_json JSON NOT NULL,
    updated_unix_ms BIGINT NOT NULL,
    INDEX idx_source_schema_version (schema_version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
