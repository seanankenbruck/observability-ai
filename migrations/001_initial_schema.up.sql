-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Create services table
CREATE TABLE services (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    namespace VARCHAR(255) NOT NULL DEFAULT 'default',
    description TEXT,
    labels JSONB DEFAULT '{}',
    metric_names JSONB DEFAULT '[]',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT services_name_namespace_unique UNIQUE (name, namespace)
);

-- Create metrics table
CREATE TABLE metrics (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL CHECK (type IN ('counter', 'gauge', 'histogram', 'summary')),
    description TEXT,
    labels JSONB DEFAULT '{}',
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT metrics_name_service_unique UNIQUE (name, service_id)
);

-- Create query_embeddings table for semantic search
CREATE TABLE query_embeddings (
    id UUID PRIMARY KEY,
    query_text TEXT NOT NULL,
    embedding vector(1536), -- OpenAI text-embedding-3-small dimension
    promql_template TEXT NOT NULL,
    success_count INTEGER DEFAULT 0,
    failure_count INTEGER DEFAULT 0,
    avg_execution_time_ms INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT query_embeddings_query_unique UNIQUE (query_text)
);

-- Create query_history table for analytics and learning
CREATE TABLE query_history (
    id UUID PRIMARY KEY,
    user_id VARCHAR(255),
    natural_query TEXT NOT NULL,
    generated_promql TEXT NOT NULL,
    intent_type VARCHAR(50),
    service_name VARCHAR(255),
    success BOOLEAN NOT NULL,
    execution_time_ms INTEGER,
    error_message TEXT,
    confidence_score FLOAT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create indexes for performance
CREATE INDEX idx_services_name ON services USING btree (name);
CREATE INDEX idx_services_namespace ON services USING btree (namespace);
CREATE INDEX idx_services_labels ON services USING gin (labels);
CREATE INDEX idx_services_created_at ON services USING btree (created_at);

CREATE INDEX idx_metrics_name ON metrics USING btree (name);
CREATE INDEX idx_metrics_type ON metrics USING btree (type);
CREATE INDEX idx_metrics_service_id ON metrics USING btree (service_id);
CREATE INDEX idx_metrics_labels ON metrics USING gin (labels);

-- Vector similarity search index (HNSW for best performance)
CREATE INDEX idx_query_embeddings_vector ON query_embeddings
USING hnsw (embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 64);

CREATE INDEX idx_query_embeddings_query_text ON query_embeddings USING btree (query_text);
CREATE INDEX idx_query_embeddings_created_at ON query_embeddings USING btree (created_at);

CREATE INDEX idx_query_history_user_id ON query_history USING btree (user_id);
CREATE INDEX idx_query_history_created_at ON query_history USING btree (created_at);
CREATE INDEX idx_query_history_success ON query_history USING btree (success);
CREATE INDEX idx_query_history_intent_type ON query_history USING btree (intent_type);

-- Create updated_at trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create triggers to automatically update updated_at
CREATE TRIGGER update_services_updated_at
    BEFORE UPDATE ON services
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_metrics_updated_at
    BEFORE UPDATE ON metrics
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_query_embeddings_updated_at
    BEFORE UPDATE ON query_embeddings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();