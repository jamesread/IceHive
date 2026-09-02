ALTER TABLE icehive_heartbeats
    ADD COLUMN version VARCHAR(255) NOT NULL DEFAULT '' AFTER latest_heartbeat_unix_ms;
