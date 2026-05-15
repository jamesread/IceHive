CREATE TABLE IF NOT EXISTS icehive_collection_sources (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    collector_type VARCHAR(128) NOT NULL,
    source_spec TEXT NOT NULL,
    interval_seconds INT NOT NULL,
    enabled TINYINT(1) NOT NULL DEFAULT 1,
    last_run_unix_ms BIGINT NULL,
    last_success_unix_ms BIGINT NULL,
    last_error TEXT NULL,
    next_due_unix_ms BIGINT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_collection_sources_collector (collector_type),
    INDEX idx_collection_sources_enabled_due (enabled, next_due_unix_ms)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
