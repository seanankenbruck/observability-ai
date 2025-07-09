-- This script is run when the PostgreSQL container starts
-- It ensures the pgvector extension is available

-- Create the vector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Verify the extension is installed
SELECT extname, extversion FROM pg_extension WHERE extname = 'vector';

-- Grant necessary permissions
GRANT ALL PRIVILEGES ON DATABASE observability_ai TO obs_ai;