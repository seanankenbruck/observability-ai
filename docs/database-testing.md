# Database Layer Testing Guide

This guide helps you test the PostgreSQL implementation of the semantic mapper with example data.

## Prerequisites

- Docker and Docker Compose
- Go 1.21+
- Make (optional, for convenience commands)

## Quick Start

1. **Start the development environment:**
   ```bash
   make start
   # OR manually:
   # docker-compose -f docker-compose.test.yml up -d
   # go run cmd/test-db/main.go
   ```

2. **The test will automatically:**
   - Create and configure PostgreSQL with pgvector
   - Run database migrations
   - Create example services and metrics
   - Test all database operations
   - Show a summary of created data

## What Gets Created

### Example Services
- **user-service** (production) - Backend team, critical tier
- **payment-service** (production) - Payments team, critical tier
- **notification-service** (production) - Platform team, standard tier
- **analytics-service** (staging) - Data team, experimental tier

### Example Metrics (per service)
- `http_requests_total` (counter) - HTTP request counts
- `http_request_duration_seconds` (histogram) - Request latency
- `database_connections_active` (gauge) - DB connection pool
- `queue_messages_processed_total` (counter) - Message processing

### Example Query Embeddings
- "show error rate for user-service"
- "display latency for payment service"
- "throughput of notification service"

## Manual Testing Commands

### Database Inspection
```bash
# Connect to PostgreSQL
make psql

# List services
make db-services

# List metrics
make db-metrics

# List query embeddings
make db-embeddings
```

### Testing Individual Components
```bash
# Test just the database setup
go run cmd/test-db/main.go

# Check database health
docker-compose -f docker-compose.test.yml exec postgres pg_isready
```

## Database Schema Overview

### Core Tables
- **services** - Service definitions with metadata
- **metrics** - Metric definitions linked to services
- **query_embeddings** - Vector embeddings for semantic search
- **query_history** - Query execution history and analytics

### Key Features
- **Vector Similarity Search** - Uses pgvector with HNSW indexing
- **JSON Support** - Flexible metadata storage with JSONB
- **Foreign Keys** - Data integrity with cascade deletes
- **Automatic Timestamps** - Created/updated tracking

## Expected Test Output

```
=== Observability AI Database Test ===
Connecting to database: obs_ai@localhost:5432/observability_ai

1. Testing database creation and migration...
✓ Database setup successful

2. Initializing semantic mapper...
✓ Semantic mapper initialized

3. Creating example services...
  Created service: user-service (production)
  Created service: payment-service (production)
  Created service: notification-service (production)
  Created service: analytics-service (staging)
✓ Created 4 services

4. Creating example metrics...
  Created 4 metrics for user-service
  Created 4 metrics for payment-service
  Created 4 metrics for notification-service
  Created 4 metrics for analytics-service
✓ Created 16 metrics

5. Testing service queries...
  Found 4 services
  Retrieved service by name: user-service
✓ Service queries working

6. Testing metric queries...
  Found 4 metrics for service user-service
✓ Metric queries working

7. Testing query embeddings...
  Stored embedding for: show error rate for user-service
  Stored embedding for: display latency for payment service
  Stored embedding for: throughput of notification service
  Found 3 similar queries
    - Similarity 1.000: show error rate for user-service
    - Similarity 0.999: display latency for payment service
    - Similarity 0.998: throughput of notification service
✓ Query embeddings working

8. Testing search functionality...
  Search for 'user' found 1 services
  Search for 'production' found 3 services
✓ Search functionality working

🎉 All database tests passed successfully!
```

## Understanding the Components

### 1. Semantic Mapper Interface
The `semantic.Mapper` interface defines all database operations:
- Service CRUD operations
- Metric management
- Vector similarity search for queries

### 2. PostgreSQL Implementation
The `PostgresMapper` implements the interface using:
- Connection pooling for performance
- JSON storage for flexible metadata
- Vector embeddings for semantic search
- Transaction support for data integrity

### 3. Database Migrations
Managed by golang-migrate with:
- Automatic schema versioning
- Rollback capability
- Extension management (pgvector)

### 4. Query Embeddings
Vector storage for semantic similarity:
- 1536-dimensional vectors (OpenAI compatible)
- Cosine similarity search
- HNSW indexing for performance

## Troubleshooting

### Common Issues

1. **Connection refused**
   ```bash
   # Check if PostgreSQL is running
   docker-compose -f docker-compose.test.yml ps

   # View logs
   make logs
   ```

2. **Permission denied**
   ```bash
   # Reset the database
   make clean
   make setup
   ```

3. **Vector extension missing**
   ```bash
   # Verify pgvector is installed
   make psql
   \dx vector
   ```

### Clean Reset
```bash
make clean  # Removes all containers and volumes
make start  # Fresh setup with test data
```

## Next Steps

After verifying the database layer works:

1. **Implement LLM Client** - Add OpenAI integration for real embeddings
2. **Add Query Processor Tests** - Test the intent classification and safety checks
3. **Create Web Interface** - Build the chat UI
4. **Add Monitoring** - Prometheus metrics and health checks
5. **Production Deployment** - Helm charts and Kubernetes manifests

The database layer provides a solid foundation for the semantic query processing system!