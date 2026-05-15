ALTER TABLE icehive_collection_sources
  ADD COLUMN cron_line VARCHAR(256) NOT NULL DEFAULT '0 * * * *' AFTER source_spec;

UPDATE icehive_collection_sources SET cron_line = CASE
  WHEN interval_seconds < 60 THEN '0 * * * *'
  WHEN interval_seconds = 60 THEN '* * * * *'
  WHEN interval_seconds = 3600 THEN '0 * * * *'
  WHEN interval_seconds >= 86400 THEN '0 0 * * *'
  ELSE CONCAT('*/', GREATEST(1, FLOOR(interval_seconds / 60)), ' * * * *')
END;

ALTER TABLE icehive_collection_sources DROP COLUMN interval_seconds;
