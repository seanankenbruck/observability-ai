-- Rollback migration: Remove service discovery fields

-- Drop indexes
DROP INDEX IF EXISTS idx_services_namespace;
DROP INDEX IF EXISTS idx_services_name_namespace;
DROP INDEX IF EXISTS idx_metrics_service_id;
DROP INDEX IF EXISTS idx_metrics_name;

-- Remove columns
ALTER TABLE services DROP COLUMN IF EXISTS namespace;
ALTER TABLE services DROP COLUMN IF EXISTS discovered_at;
ALTER TABLE services DROP COLUMN IF EXISTS last_seen;

-- Note: We don't drop the metrics table as it may have been created in the initial migration
