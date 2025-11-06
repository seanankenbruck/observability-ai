# Troubleshooting Guide

Comprehensive guide to diagnosing and resolving common issues with Observability AI.

---

## Table of Contents

- [Quick Diagnosis](#quick-diagnosis)
- [Startup Issues](#startup-issues)
- [Database Issues](#database-issues)
- [Redis Issues](#redis-issues)
- [Claude API Issues](#claude-api-issues)
- [Prometheus/Mimir Issues](#prometheusmimir-issues)
- [Authentication Issues](#authentication-issues)
- [Query Issues](#query-issues)
- [Performance Issues](#performance-issues)
- [Frontend Issues](#frontend-issues)
- [Network Issues](#network-issues)
- [Docker Issues](#docker-issues)
- [Advanced Debugging](#advanced-debugging)

---

## Quick Diagnosis

### Is Everything Running?

```bash
# Check health endpoint
curl http://localhost:8080/health
```

**Healthy Response:**
```json
{
  "status": "healthy",
  "timestamp": "2025-01-15T10:00:00Z"
}
```

**If health check fails**, proceed to component-specific sections below.

---

### Component Health Check

```bash
# Detailed health check
curl http://localhost:8080/api/v1/health
```

**Response:**
```json
{
  "status": "healthy",
  "components": {
    "database": "healthy",
    "redis": "healthy",
    "claude_api": "healthy",
    "prometheus": "healthy"
  }
}
```

**If any component shows unhealthy**, jump to that component's section.

---

### Check Logs

```bash
# All services
make logs

# Specific service
docker logs observability-ai-backend-1
docker logs observability-ai-postgres-1
docker logs observability-ai-redis-1

# Follow logs in real-time
docker logs -f observability-ai-backend-1
```

---

## Startup Issues

### Problem: Application Won't Start

**Symptoms:**
- Container immediately exits
- "Error: fatal error" in logs
- Backend not responding

**Diagnosis:**
```bash
# Check logs
docker logs observability-ai-backend-1

# Check if process is running
docker ps | grep observability-ai

# Check exit code
docker inspect observability-ai-backend-1 | grep ExitCode
```

**Common Causes & Solutions:**

#### 1. Missing CLAUDE_API_KEY

**Error:**
```
FATAL: CLAUDE_API_KEY environment variable not set
```

**Solution:**
```bash
# Check if set
grep CLAUDE_API_KEY .env

# Add it
echo "CLAUDE_API_KEY=sk-ant-your-key-here" >> .env

# Restart
make restart
```

#### 2. Database Connection Failed

**Error:**
```
FATAL: failed to connect to database: connection refused
```

**Solution:**
```bash
# Check PostgreSQL is running
docker ps | grep postgres

# Check PostgreSQL health
make health-check

# If not running, start it
make setup

# Wait for PostgreSQL to be ready
sleep 15
make health-check
```

#### 3. Port Already in Use

**Error:**
```
FATAL: listen tcp :8080: bind: address already in use
```

**Solution:**
```bash
# Find what's using port 8080
lsof -i :8080

# Kill the process
kill -9 $(lsof -t -i:8080)

# Or use a different port
echo "PORT=8081" >> .env
make restart
```

---

### Problem: Migrations Failed

**Symptoms:**
- "Migration failed" in logs
- Database tables don't exist
- "relation does not exist" errors

**Diagnosis:**
```bash
# Check migration status
make psql
# Then in psql:
SELECT * FROM schema_migrations;
```

**Solution:**
```bash
# Re-run migrations
make migrate

# If that fails, reset database (⚠️ destroys data!)
make clean
make setup
make migrate
```

**Manual Migration:**
```bash
# Connect to database
make psql

# Run migrations manually
\i migrations/001_initial_schema.sql
\i migrations/002_add_api_keys.sql
\i migrations/003_add_metrics.sql
```

---

## Database Issues

### Problem: "Connection Refused"

**Error:**
```
pq: connection refused
dial tcp 127.0.0.1:5433: connect: connection refused
```

**Diagnosis:**
```bash
# Check if PostgreSQL is running
docker ps | grep postgres

# Check PostgreSQL logs
docker logs observability-ai-postgres-1

# Try connecting directly
psql -h localhost -p 5433 -U obs_ai -d observability_ai
```

**Solutions:**

#### PostgreSQL Not Running
```bash
# Start PostgreSQL
docker-compose up -d postgres

# Or use make
make setup
```

#### Wrong Port
```bash
# Check configured port
grep DB_PORT .env

# Default for local: 5433
# Default for Docker: 5432

# Update .env
# For local development:
DB_PORT=5433

# For backend in Docker:
DB_PORT=5432
```

#### Wrong Host
```bash
# For local development:
DB_HOST=localhost

# For backend in Docker:
DB_HOST=postgres
```

---

### Problem: "Authentication Failed"

**Error:**
```
pq: password authentication failed for user "obs_ai"
FATAL: role "obs_ai" does not exist
```

**Solutions:**

#### Wrong Password
```bash
# Check password in .env
grep DB_PASSWORD .env

# Check PostgreSQL password
# Default: changeme

# Reset if needed (destroys data!)
make clean
make setup
```

#### User Doesn't Exist
```bash
# Connect as superuser
docker exec -it observability-ai-postgres-1 psql -U postgres

# Create user
CREATE USER obs_ai WITH PASSWORD 'changeme';
CREATE DATABASE observability_ai OWNER obs_ai;
GRANT ALL PRIVILEGES ON DATABASE observability_ai TO obs_ai;
```

---

### Problem: "Too Many Connections"

**Error:**
```
pq: sorry, too many clients already
FATAL: remaining connection slots are reserved
```

**Diagnosis:**
```bash
# Check active connections
make psql
SELECT count(*) FROM pg_stat_activity;
```

**Solutions:**

#### Increase max_connections
```bash
# Edit PostgreSQL config
docker exec -it observability-ai-postgres-1 bash
echo "max_connections = 200" >> /var/lib/postgresql/data/postgresql.conf

# Restart PostgreSQL
docker-compose restart postgres
```

#### Fix Connection Leaks
- Check application for connection leaks
- Ensure connections are properly closed
- Check connection pool settings

---

### Problem: Database Performance Issues

**Symptoms:**
- Slow queries
- High CPU usage on PostgreSQL
- Timeouts

**Diagnosis:**
```bash
# Check slow queries
make psql

SELECT pid, now() - pg_stat_activity.query_start AS duration, query
FROM pg_stat_activity
WHERE state = 'active' AND now() - pg_stat_activity.query_start > interval '5 seconds'
ORDER BY duration DESC;
```

**Solutions:**

#### Add Indexes
```sql
-- Index for query history
CREATE INDEX idx_query_history_user_id ON query_history(user_id);
CREATE INDEX idx_query_history_created_at ON query_history(created_at);

-- Index for services
CREATE INDEX idx_services_name ON services(name);

-- Index for metrics
CREATE INDEX idx_metrics_name ON metrics(name);
```

#### Vacuum Database
```bash
make psql
VACUUM ANALYZE;
```

---

## Redis Issues

### Problem: "Connection Refused"

**Error:**
```
dial tcp 127.0.0.1:6379: connect: connection refused
redis: connection refused
```

**Diagnosis:**
```bash
# Check if Redis is running
docker ps | grep redis

# Test connection
redis-cli -h localhost -p 6379 -a changeme ping
```

**Solutions:**

#### Redis Not Running
```bash
# Start Redis
docker-compose up -d redis

# Or use make
make setup
```

#### Wrong Address
```bash
# Check .env
grep REDIS_ADDR .env

# For local development:
REDIS_ADDR=localhost:6379

# For backend in Docker:
REDIS_ADDR=redis:6379
```

---

### Problem: "NOAUTH Authentication Required"

**Error:**
```
NOAUTH Authentication required
ERR invalid password
```

**Solutions:**

#### Wrong Password
```bash
# Check password in .env
grep REDIS_PASSWORD .env

# Default: changeme

# Test connection with password
redis-cli -h localhost -p 6379 -a changeme ping
```

#### No Password Set
```bash
# If Redis has no password, leave empty
REDIS_PASSWORD=
```

---

### Problem: Cache Not Working

**Symptoms:**
- All queries show `"cached": false`
- Slow query performance
- Redis shows no keys

**Diagnosis:**
```bash
# Check Redis keys
redis-cli -h localhost -p 6379 -a changeme

# List all keys
KEYS *

# Check specific query cache
KEYS query:*
```

**Solutions:**

#### Redis Not Connected
```bash
# Check Redis health in application
curl http://localhost:8080/api/v1/health

# Should show:
# "redis": "healthy"
```

#### Cache Expiration Too Short
```bash
# Queries expire after 5 minutes by default
# Increase in code or wait for cache to populate
```

---

## Claude API Issues

### Problem: "Invalid API Key"

**Error:**
```
anthropic: invalid API key
401 Unauthorized
authentication_error: invalid x-api-key
```

**Solutions:**

#### Check API Key Format
```bash
# Valid format starts with: sk-ant-api03-
grep CLAUDE_API_KEY .env

# Should look like:
CLAUDE_API_KEY=sk-ant-api03-xxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

#### Verify API Key
```bash
# Test API key directly
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: $CLAUDE_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model":"claude-3-haiku-20240307","max_tokens":1024,"messages":[{"role":"user","content":"test"}]}'
```

#### Get New API Key
1. Go to https://console.anthropic.com/
2. Navigate to API Keys
3. Generate new key
4. Update .env file

---

### Problem: "Rate Limit Exceeded"

**Error:**
```
rate_limit_error: rate limit exceeded
429 Too Many Requests
```

**Diagnosis:**
```bash
# Check your usage
# Visit: https://console.anthropic.com/account/billing

# Check rate limit in response headers
curl -i http://localhost:8080/query \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query": "test"}'
```

**Solutions:**

#### Wait and Retry
- Claude API has rate limits per minute
- Wait 60 seconds and try again

#### Upgrade Plan
- Free tier: Limited requests
- Paid tier: Higher limits
- Visit: https://console.anthropic.com/account/billing

#### Implement Caching
- Queries are cached by default
- Identical queries use cache (no API call)

---

### Problem: "Insufficient Credits"

**Error:**
```
insufficient_credits: out of credits
payment_required: please add credits
```

**Solution:**
1. Go to https://console.anthropic.com/account/billing
2. Add credits to your account
3. Try query again

---

### Problem: Slow Claude API Responses

**Symptoms:**
- Queries take >10 seconds
- Timeout errors
- Slow query execution

**Diagnosis:**
```bash
# Check execution time in response
curl -X POST http://localhost:8080/query \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query": "test"}' | jq '.execution_time_ms'
```

**Solutions:**

#### Use Faster Model
```bash
# Switch to Haiku (fastest)
CLAUDE_MODEL=claude-3-haiku-20240307

# Current model (slower but more accurate)
CLAUDE_MODEL=claude-3-opus-20240229
```

#### Increase Timeout
```bash
# Default: 30 seconds
CLAUDE_API_TIMEOUT=60
```

#### Check Claude Status
- Visit: https://status.anthropic.com/
- May be experiencing service issues

---

## Prometheus/Mimir Issues

### Problem: "Connection Refused"

**Error:**
```
prometheus: connection refused
Get "http://localhost:9009/api/v1/query": dial tcp: connection refused
```

**Solutions:**

#### Prometheus/Mimir Not Running
```bash
# Check if running
curl http://localhost:9009/-/healthy

# If not running, start your Prometheus/Mimir instance
```

#### Wrong URL
```bash
# Check configured URL
grep MIMIR_URL .env

# Common URLs:
# Prometheus: http://localhost:9090
# Mimir: http://localhost:9009
# Grafana Cloud: https://prometheus-prod-xx.grafana.net

# Update .env
MIMIR_URL=http://your-prometheus:9090
```

---

### Problem: "Authentication Failed"

**Error:**
```
401 Unauthorized
403 Forbidden
authentication required
```

**Solutions:**

#### Set Authentication Type
```bash
# Check auth type
grep MIMIR_AUTH_TYPE .env

# For no auth (local):
MIMIR_AUTH_TYPE=none

# For basic auth:
MIMIR_AUTH_TYPE=basic
MIMIR_USERNAME=admin
MIMIR_PASSWORD=your-password

# For bearer token:
MIMIR_AUTH_TYPE=bearer
MIMIR_BEARER_TOKEN=your-token
```

---

### Problem: "No Services Found"

**Symptoms:**
- `/services` endpoint returns empty array
- Service discovery not working
- No metrics discovered

**Diagnosis:**
```bash
# Check if discovery is enabled
grep DISCOVERY_ENABLED .env

# Check discovery logs
docker logs observability-ai-backend-1 | grep discovery

# Test Prometheus connection
curl http://localhost:9009/api/v1/labels
```

**Solutions:**

#### Enable Discovery
```bash
# Enable in .env
DISCOVERY_ENABLED=true

# Restart
make restart
```

#### Manually Trigger Discovery
```bash
# Get admin token first
TOKEN=$(curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "your-password"}' \
  | jq -r '.token')

# Trigger discovery
curl -X POST http://localhost:8080/admin/discovery/trigger \
  -H "Authorization: Bearer $TOKEN"
```

#### Load Test Data
```bash
# Load sample services and metrics
make test-db
```

#### Check Prometheus Has Metrics
```bash
# Query Prometheus directly
curl http://localhost:9009/api/v1/label/__name__/values

# Should return list of metric names
```

---

## Authentication Issues

### Problem: "401 Unauthorized"

**Error:**
```
{
  "error": "unauthorized",
  "message": "missing or invalid token"
}
```

**Solutions:**

#### Missing Token
```bash
# Include Authorization header
curl -X POST http://localhost:8080/query \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query": "test"}'
```

#### Expired Token
```bash
# Tokens expire after 24 hours by default
# Login again to get new token
TOKEN=$(curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "your-user", "password": "your-pass"}' \
  | jq -r '.token')
```

#### Invalid Token Format
```bash
# Token should be in format: eyJhbGciOiJIUzI1NiIsInR...
# Check token
echo $TOKEN

# Should start with: eyJ
```

---

### Problem: "Invalid Credentials"

**Error:**
```
{
  "error": "invalid_credentials",
  "message": "incorrect username or password"
}
```

**Solutions:**

#### Register New User
```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "email": "test@example.com",
    "password": "securepass123"
  }'
```

#### Reset Password (Admin Required)
```bash
# Connect to database
make psql

# Reset password (bcrypt hash of "newpassword")
UPDATE users
SET password_hash = '$2a$10$...'
WHERE username = 'username';
```

---

### Problem: "Rate Limit Exceeded"

**Error:**
```
{
  "error": "rate_limit_exceeded",
  "retry_after": 3600
}
```

**Solutions:**

#### Wait for Window to Reset
- Default: 100 requests per hour
- Check X-RateLimit-Reset header
- Wait until reset time

#### Increase Rate Limit (Admin)
```bash
# Update API key rate limit
curl -X PUT http://localhost:8080/admin/api-keys/$KEY_ID \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"rate_limit": 1000}'
```

---

## Query Issues

### Problem: "Invalid PromQL Generated"

**Symptoms:**
- Query returns error
- "Invalid expression" messages
- Prometheus rejects query

**Diagnosis:**
```bash
# Check generated PromQL in response
curl -X POST http://localhost:8080/query \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query": "your query"}' | jq '.promql'

# Test PromQL directly in Prometheus
curl 'http://localhost:9009/api/v1/query?query=YOUR_PROMQL'
```

**Solutions:**

#### Try Different Phrasing
```bash
# Instead of:
"Show CPU"

# Try:
"What is the CPU usage for the auth service?"
```

#### Use Better Model
```bash
# Switch to Sonnet or Opus for better accuracy
CLAUDE_MODEL=claude-3-sonnet-20240229
```

#### Report Issue
- Query works manually but AI generates wrong syntax
- Open GitHub issue with example query

---

### Problem: "No Results Returned"

**Symptoms:**
- Query succeeds but returns empty results
- `"result": []` in response

**Diagnosis:**
```bash
# Check if metrics exist
curl http://localhost:8080/metrics \
  -H "Authorization: Bearer $TOKEN"

# Query Prometheus directly
curl 'http://localhost:9009/api/v1/query?query=up'
```

**Solutions:**

#### Metric Doesn't Exist
```bash
# Service discovery may not have run yet
# Trigger manually
curl -X POST http://localhost:8080/admin/discovery/trigger \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

#### Wrong Time Range
```bash
# Query with specific time range
curl -X POST http://localhost:8080/query \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "query": "CPU usage",
    "start": "2025-01-01T00:00:00Z",
    "end": "2025-01-15T00:00:00Z"
  }'
```

---

## Performance Issues

### Problem: Slow Queries

**Symptoms:**
- Queries take >5 seconds
- High execution_time_ms values
- Users complaining about slowness

**Diagnosis:**
```bash
# Check query execution time
curl -X POST http://localhost:8080/query \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query": "test"}' | jq '.execution_time_ms'

# Check individual components
# Look at response for breakdown
```

**Solutions:**

#### Enable Caching
- Cache is enabled by default
- Identical queries should be fast (<100ms)

#### Use Faster Claude Model
```bash
CLAUDE_MODEL=claude-3-haiku-20240307
```

#### Optimize Prometheus Queries
- Complex PromQL can be slow
- Check Prometheus performance

#### Add Database Indexes
```sql
-- See Database Performance Issues section
```

---

### Problem: High Memory Usage

**Symptoms:**
- Backend using >1GB RAM
- OOM (out of memory) errors
- Container killed

**Diagnosis:**
```bash
# Check memory usage
docker stats observability-ai-backend-1

# Check for memory leaks in logs
docker logs observability-ai-backend-1 | grep -i "memory"
```

**Solutions:**

#### Increase Memory Limit
```yaml
# docker-compose.yml
services:
  backend:
    mem_limit: 2g
```

#### Reduce Cache Size
- Redis cache may be too large
- Set TTL or maxmemory policy

---

## Frontend Issues

### Problem: "Cannot Connect to Backend"

**Symptoms:**
- Frontend shows connection errors
- CORS errors in browser console
- Network requests fail

**Diagnosis:**
```bash
# Open browser DevTools (F12)
# Check Console for errors
# Check Network tab for failed requests

# Verify backend is running
curl http://localhost:8080/health
```

**Solutions:**

#### Backend Not Running
```bash
# Start backend
make start-backend
```

#### Wrong API URL
```bash
# Check frontend API configuration
cat web/src/utils/api.ts

# Should be:
const API_URL = 'http://localhost:8080';
```

#### CORS Issues
- Backend should allow `localhost:3000` origin
- Check backend CORS configuration

---

## Network Issues

### Problem: DNS Resolution Failed

**Error:**
```
dial tcp: lookup postgres: no such host
getaddrinfo: Name or service not known
```

**Solutions:**

#### Use IP Address
```bash
# Instead of hostname:
DB_HOST=172.17.0.2

# Find IP:
docker inspect observability-ai-postgres-1 | grep IPAddress
```

#### Check Docker Network
```bash
# List networks
docker network ls

# Inspect network
docker network inspect observability-ai_default

# Ensure all services on same network
```

---

## Docker Issues

### Problem: "No Space Left on Device"

**Error:**
```
no space left on device
Error response from daemon: no space left on device
```

**Solutions:**

#### Clean Docker
```bash
# Remove unused data
docker system prune -a

# Remove unused volumes
docker volume prune

# Check disk usage
docker system df
```

---

### Problem: "Container Keeps Restarting"

**Symptoms:**
- Container starts then immediately stops
- Restart loop

**Diagnosis:**
```bash
# Check logs
docker logs observability-ai-backend-1

# Check restart count
docker ps -a | grep observability-ai

# Inspect container
docker inspect observability-ai-backend-1
```

**Solutions:**
- Fix configuration error causing startup failure
- Check logs for specific error
- Ensure dependencies (DB, Redis) are running

---

## Advanced Debugging

### Enable Debug Logging

```bash
# Set environment variable
export GIN_MODE=debug
export LOG_LEVEL=debug

# Restart application
make restart

# Check logs
make logs
```

### Interactive Debugging

```bash
# Connect to database
make psql

# Connect to Redis
make redis-cli

# Enter container
docker exec -it observability-ai-backend-1 bash
```

### Network Debugging

```bash
# Test connectivity from container
docker exec -it observability-ai-backend-1 bash

# Inside container:
curl http://postgres:5432
curl http://redis:6379
curl http://prometheus:9090
```

### Check Resource Usage

```bash
# Monitor resources
docker stats

# Check container resource limits
docker inspect observability-ai-backend-1 | grep -A 10 Resources
```

---

## Getting Help

If you've tried everything and still can't resolve your issue:

1. **Check logs thoroughly**:
   ```bash
   make logs > debug.log
   ```

2. **Gather information**:
   - OS and Docker version
   - Configuration (sanitized, no passwords!)
   - Error messages
   - Steps to reproduce

3. **Search existing issues**:
   - [GitHub Issues](https://github.com/seanankenbruck/observability-ai/issues)

4. **Ask for help**:
   - [Open a new issue](https://github.com/seanankenbruck/observability-ai/issues/new)
   - [GitHub Discussions](https://github.com/seanankenbruck/observability-ai/discussions)

---

## Further Reading

- **[CONFIGURATION.md](CONFIGURATION.md)** - Configuration reference
- **[ARCHITECTURE.md](ARCHITECTURE.md)** - System architecture
- **[FAQ.md](FAQ.md)** - Frequently asked questions
- **[QUICKSTART.md](../QUICKSTART.md)** - Getting started guide

---

**Still stuck?** [Open an issue](https://github.com/seanankenbruck/observability-ai/issues/new) with details!
