ALTER TABLE icehive_collection_sources
  ADD COLUMN interval_seconds INT NOT NULL DEFAULT 3600 AFTER source_spec;

UPDATE icehive_collection_sources SET interval_seconds = 3600;

ALTER TABLE icehive_collection_sources DROP COLUMN cron_line;
