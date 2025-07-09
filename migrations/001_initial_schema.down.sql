-- Drop triggers
DROP TRIGGER IF EXISTS update_services_updated_at ON services;
DROP TRIGGER IF EXISTS update_metrics_updated_at ON metrics;
DROP TRIGGER IF EXISTS update_query_embeddings_updated_at ON query_embeddings;

-- Drop trigger function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_services_name;
DROP INDEX IF EXISTS idx_services_namespace;
DROP INDEX IF EXISTS idx_services_labels;
DROP INDEX IF EXISTS idx_services_created_at;

DROP INDEX IF EXISTS idx_metrics_name;
DROP INDEX IF EXISTS idx_metrics_type;
DROP INDEX IF EXISTS idx_metrics_service_id;
DROP INDEX IF EXISTS idx_metrics_labels;

DROP INDEX IF EXISTS idx_query_embeddings_vector;
DROP INDEX IF EXISTS idx_query_embeddings_query_text;
DROP INDEX IF EXISTS idx_query_embeddings_created_at;

DROP INDEX IF EXISTS idx_query_history_user_id;
DROP INDEX IF EXISTS idx_query_history_created_at;
DROP INDEX IF EXISTS idx_query_history_success;
DROP INDEX IF EXISTS idx_query_history_intent_type;

-- Drop tables (order matters due to foreign keys)
DROP TABLE IF EXISTS query_history;
DROP TABLE IF EXISTS query_embeddings;
DROP TABLE IF EXISTS metrics;
DROP TABLE IF EXISTS services;

-- Drop extension (optional - might be used by other applications)
-- DROP EXTENSION IF EXISTS vector;