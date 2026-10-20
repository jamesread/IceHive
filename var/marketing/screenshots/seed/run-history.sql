-- Fictional run history so the sources page shows a healthy pipeline
-- (only collector-testdata actually runs in the marketing stack).
SET @now = UNIX_TIMESTAMP() * 1000;
UPDATE icehive_collection_sources SET last_run_unix_ms = @now - 60000, last_success_unix_ms = @now - 60000 WHERE id = 'demo-animals';
UPDATE icehive_collection_sources SET last_run_unix_ms = @now - 1260000, last_success_unix_ms = @now - 1260000 WHERE id = 'demo-github-api';
UPDATE icehive_collection_sources SET last_run_unix_ms = @now - 540000, last_success_unix_ms = @now - 540000 WHERE id = 'demo-rss-news';
