-- Migration: Add service discovery fields
-- Created: 2025-11-03

-- Add namespace and labels to services table if not exists
ALTER TABLE services ADD COLUMN IF NOT EXISTS namespace VARCHAR(255) DEFAULT 'default';
ALTER TABLE services ADD COLUMN IF NOT EXISTS discovered_at TIMESTAMP;
ALTER TABLE services ADD COLUMN IF NOT EXISTS last_seen TIMESTAMP;

-- Add indexes for faster lookups
CREATE INDEX IF NOT EXISTS idx_services_namespace ON services(namespace);
CREATE INDEX IF NOT EXISTS idx_services_name_namespace ON services(name, namespace);

-- Update existing services to have default namespace
UPDATE services SET namespace = 'default' WHERE namespace IS NULL;

-- Add metrics table if it doesn't exist (it should already exist from initial schema)
CREATE TABLE IF NOT EXISTS metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    service_id UUID REFERENCES services(id) ON DELETE CASCADE,
    type VARCHAR(50),
    description TEXT,
    labels JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(name, service_id)
);

CREATE INDEX IF NOT EXISTS idx_metrics_service_id ON metrics(service_id);
CREATE INDEX IF NOT EXISTS idx_metrics_name ON metrics(name);
