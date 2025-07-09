#!/bin/bash

# Integration Test Script

set -e

echo "=== Query Processor Integration Test Setup ==="

# Check required environment variables
required_vars=("CLAUDE_API_KEY", "CLAUDE_MODEL", "DB_HOST" "DB_NAME" "DB_USER" "DB_PASSWORD")

for var in "${required_vars[@]}"; do
    if [ -z "${!var}" ]; then
        echo "❌ Required environment variable $var is not set"
        echo ""
        echo "Please set the following environment variables:"
        echo "export CLAUDE_API_KEY='your-claude-api-key'"
        echo "export CLAUDE_MODEL='your-target-model'"
        echo "export DB_HOST='localhost'"
        echo "export DB_PORT='5432'"
        echo "export DB_NAME='observability_ai'"
        echo "export DB_USER='obs_ai'"
        echo "export DB_PASSWORD='changeme'"
        echo "export REDIS_ADDR='localhost:6379'"
        exit 1
    fi
done

echo "✓ All required environment variables are set"

# Check if database is accessible
echo "Checking database connection..."
if command -v psql >/dev/null 2>&1; then
    if PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1;" >/dev/null 2>&1; then
        echo "✓ Database connection successful"
    else
        echo "❌ Cannot connect to database"
        echo "Make sure PostgreSQL is running with: make setup"
        exit 1
    fi
else
    echo "⚠️  psql not found, skipping database connectivity check"
fi

# Check if Redis is accessible
echo "Checking Redis connection..."
if command -v redis-cli >/dev/null 2>&1; then
    if redis-cli -h localhost -p 6379 ping >/dev/null 2>&1; then
        echo "✓ Redis connection successful"
    else
        echo "❌ Cannot connect to Redis"
        echo "Make sure Redis is running with: make setup"
        exit 1
    fi
else
    echo "⚠️  redis-cli not found, skipping Redis connectivity check"
fi

# Build the test program
echo "Building integration test..."
go build -o bin/test-integration cmd/test-integration/main.go
echo "✓ Integration test built"

# Run the test
echo ""
echo "Starting integration test..."
echo "This will make real API calls to Claude and may take 30-60 seconds..."
echo ""
echo "="*60

./bin/test-integration

echo ""
echo "Integration test completed!"