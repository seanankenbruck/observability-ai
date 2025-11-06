# Quick Start Guide

Get Observability AI running in under 5 minutes! ⏱️

---

## What You'll Accomplish

By the end of this guide, you'll have:
- ✅ Observability AI running locally
- ✅ Successfully executed your first natural language query
- ✅ Verified all components are healthy
- ✅ Understanding of how to query your metrics

**Total Time: 5-10 minutes**

---

## Prerequisites

**Required:**
- ✅ Docker and Docker Compose installed ([Get Docker](https://docs.docker.com/get-docker/))
- ✅ A Claude API key from [Anthropic Console](https://console.anthropic.com/)

**Optional (for local development):**
- Go 1.21+ ([Download](https://go.dev/dl/))
- Node.js 18+ and npm ([Download](https://nodejs.org/))

**Check Your Prerequisites:**
```bash
# Verify Docker is installed
docker --version
# Expected: Docker version 20.10.0 or higher

docker-compose --version
# Expected: Docker Compose version 1.29.0 or higher
```

---

## Step 1: Clone and Configure ⏱️ ~2 minutes

```bash
# 1. Clone the repository
git clone https://github.com/seanankenbruck/observability-ai.git
cd observability-ai

# 2. Copy the example environment file
cp .env.example .env

# 3. Edit .env and add your Claude API key
# You can use any text editor (nano, vim, vscode, etc.)
nano .env
```

**Update this line in .env:**
```bash
CLAUDE_API_KEY=your-api-key-here
```
**Change to:**
```bash
CLAUDE_API_KEY=sk-ant-api03-your-actual-key-here
```

### ✅ Verification

Your `.env` file should now exist and contain your API key:

```bash
# Check the file exists
ls -la .env
# Expected: -rw-r--r-- 1 user user 1234 Jan 15 10:00 .env

# Verify your API key is set (without showing the full key)
grep "CLAUDE_API_KEY=sk-ant" .env
# Expected: CLAUDE_API_KEY=sk-ant-api03-... (your key)
```

**✅ Success Indicator:** You see your API key starting with `sk-ant-` in the grep output.

**❌ Common Issues:**
- **File not found**: Make sure you're in the `observability-ai` directory
- **API key not set**: The key should start with `sk-ant-api03-` or similar
- **Permission denied**: Try `chmod 644 .env`

---

## Step 2: Choose Your Setup Method

Pick the method that best fits your needs:

| Method | Best For | Time | Components |
|--------|----------|------|------------|
| **Method A: Full Docker** | Quick demo, first-time users | ~3 min | Everything in Docker |
| **Method B: Local Development** | Active development, hot-reload | ~4 min | Backend + Frontend local |

---

### Method A: Full Docker Setup ⏱️ ~3 minutes

**Best for:** Quick demos, if you don't want to install Go/Node, or first-time exploration.

```bash
# Start everything with Docker
make start-dev-docker
```

**What's happening:**
```
1. Pulling Docker images... (1-2 minutes)
2. Starting PostgreSQL...
3. Starting Redis...
4. Running database migrations...
5. Starting backend API...
```

**Expected output:**
```
[+] Running 3/3
 ✔ Container observability-ai-postgres-1  Started
 ✔ Container observability-ai-redis-1     Started
 ✔ Container observability-ai-backend-1   Started
```

### ✅ Verification (Method A)

```bash
# Check all containers are running
docker-compose ps
# Expected: All containers should show "Up" status

# Test the health endpoint
curl http://localhost:8080/health
```

**Expected response:**
```json
{
  "status": "healthy",
  "timestamp": "2025-01-15T10:00:00Z"
}
```

**✅ Success Indicator:** You see `"status": "healthy"` in the response.

**❌ Common Issues:**
- **Port 8080 already in use**: Run `lsof -i :8080` to find what's using it
- **PostgreSQL not ready**: Wait 10-15 seconds and try again
- **Connection refused**: Check logs with `make logs`

**Access Points:**
- 🌐 Backend API: http://localhost:8080
- 🏥 Health Check: http://localhost:8080/health

**Skip to [Step 3](#step-3-test-your-first-query--1-minute)** ⬇️

---

### Method B: Local Development ⏱️ ~4 minutes

**Best for:** Active development with hot-reload for backend and frontend.

**Requires:** Go 1.21+ and Node.js 18+ installed

```bash
# 1. Start PostgreSQL and Redis in Docker
make setup
```

**Expected output:**
```
[+] Running 2/2
 ✔ Container observability-ai-postgres-1  Started
 ✔ Container observability-ai-redis-1     Started
```

**Wait for PostgreSQL to be ready (~10 seconds):**
```bash
# Check PostgreSQL health
make health-check
```

**Expected output:**
```
PostgreSQL is ready!
```

```bash
# 2. Run database migrations
make migrate
```

**Expected output:**
```
Running migrations...
✓ Applied migration 001_initial_schema.sql
✓ Applied migration 002_add_api_keys.sql
✓ Applied migration 003_add_metrics.sql
Migrations complete!
```

```bash
# 3. (Optional) Load test data
make test-db
```

**Expected output:**
```
Loading test data...
✓ Created 5 test services
✓ Created 20 test metrics
✓ Created admin user (username: admin)
Test data loaded!
```

**📝 Important:** Note the admin credentials shown in the output!

```bash
# 4. Start backend + frontend
make dev
```

**Expected output:**
```
Starting backend on :8080...
Starting frontend on :3000...

Backend: ✓ Server started
Frontend: ✓ Development server running
```

### ✅ Verification (Method B)

**Test backend:**
```bash
curl http://localhost:8080/health
```

**Expected response:**
```json
{
  "status": "healthy",
  "components": {
    "database": "healthy",
    "redis": "healthy",
    "claude_api": "healthy"
  }
}
```

**Test frontend (open in browser):**
- 🌐 Frontend: http://localhost:3000
- Should see the Observability AI login/query interface

**✅ Success Indicators:**
- ✅ Backend returns `"status": "healthy"`
- ✅ Frontend loads in your browser
- ✅ No error messages in terminal

**❌ Common Issues:**
- **"go: command not found"**: Install Go from https://go.dev/dl/
- **"npm: command not found"**: Install Node.js from https://nodejs.org/
- **Port 5433 in use**: Another PostgreSQL instance might be running
- **Frontend won't start**: Try `cd web && npm install && npm run dev`

**Access Points:**
- 🌐 Frontend: http://localhost:3000
- 🔧 Backend API: http://localhost:8080
- 🏥 Health Check: http://localhost:8080/health

---

## Step 3: Test Your First Query ⏱️ ~1 minute

Now let's verify everything works by running some queries!

### First: Check System Health

```bash
curl http://localhost:8080/health
```

**Expected response:**
```json
{
  "status": "healthy",
  "timestamp": "2025-01-15T10:00:00Z"
}
```

### Create a User Account

Before querying, you need to register and get a token:

```bash
# Register a new user
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "email": "test@example.com",
    "password": "securepassword123"
  }'
```

**Expected response:**
```json
{
  "message": "user registered successfully",
  "user_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### Login to Get Token

```bash
# Login (use admin if you ran 'make test-db', or your registered user)
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "securepassword123"
  }'
```

**Expected response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "username": "testuser",
    "roles": ["user"]
  },
  "expires_at": "2025-01-16T10:00:00Z"
}
```

**📝 Save your token!** You'll need it for subsequent requests.

```bash
# Store the token in a variable (easier for testing)
TOKEN="your-token-from-above"
```

### Your First Natural Language Query!

```bash
# Query with natural language
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query": "What is the CPU usage for the auth service?"}'
```

**Expected response:**
```json
{
  "query": "What is the CPU usage for the auth service?",
  "promql": "rate(container_cpu_usage_seconds_total{service=\"auth\"}[5m])",
  "explanation": "This query calculates the CPU usage rate for the auth service over a 5-minute window.",
  "results": {
    "status": "success",
    "data": {
      "resultType": "vector",
      "result": [
        {
          "metric": {
            "service": "auth",
            "pod": "auth-7d9f4c5b-xk2p9"
          },
          "value": [1704121200, "0.45"]
        }
      ]
    }
  },
  "cached": false,
  "execution_time_ms": 523
}
```

**✅ Success Indicator:** You receive a response with `promql`, `explanation`, and `results` fields!

### Try More Queries

```bash
# List available services
curl http://localhost:8080/services \
  -H "Authorization: Bearer $TOKEN"
```

**Expected response:**
```json
{
  "services": [
    {
      "id": "...",
      "name": "auth",
      "namespace": "production",
      "labels": {"app": "auth", "tier": "backend"}
    },
    {
      "id": "...",
      "name": "payment",
      "namespace": "production",
      "labels": {"app": "payment", "tier": "backend"}
    }
  ],
  "count": 2
}
```

```bash
# List available metrics
curl http://localhost:8080/metrics \
  -H "Authorization: Bearer $TOKEN"
```

**Expected response:**
```json
{
  "metrics": [
    {
      "id": "...",
      "name": "container_cpu_usage_seconds_total",
      "type": "counter",
      "description": "CPU usage in seconds"
    },
    {
      "id": "...",
      "name": "http_requests_total",
      "type": "counter",
      "description": "Total HTTP requests"
    }
  ],
  "count": 20
}
```

```bash
# Check your query history
curl http://localhost:8080/history \
  -H "Authorization: Bearer $TOKEN"
```

**Expected response:**
```json
{
  "queries": [
    {
      "id": "...",
      "query": "What is the CPU usage for the auth service?",
      "promql": "rate(container_cpu_usage_seconds_total{service=\"auth\"}[5m])",
      "success": true,
      "cached": false,
      "execution_time_ms": 523,
      "created_at": "2025-01-15T10:05:00Z"
    }
  ],
  "count": 1
}
```

### ✅ Verification Checklist

At this point, you should have:
- ✅ Successfully registered a user
- ✅ Obtained a JWT token
- ✅ Executed your first natural language query
- ✅ Received PromQL and results
- ✅ Listed available services and metrics

**🎉 Congratulations!** Observability AI is working correctly!

---

## Step 4: Try More Example Queries

Now that everything is working, try these example queries:

### Resource Monitoring

```bash
# Memory usage
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query": "How much memory is the payment service using?"}'

# Network traffic
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query": "Show me network traffic for the API gateway"}'
```

### Application Performance

```bash
# Error rates
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query": "What is the error rate for the checkout service?"}'

# Latency
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query": "Show me 95th percentile latency for the API"}'
```

### Comparisons

```bash
# Service comparison
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query": "Compare CPU usage between auth and payment services"}'
```

### Troubleshooting

```bash
# What's breaking
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query": "What services have high error rates right now?"}'
```

**📚 Want more examples?** See [docs/QUERY_EXAMPLES.md](docs/QUERY_EXAMPLES.md) for 30+ query examples!

---

## Step 5: Stop Everything

When you're done testing:

```bash
# Stop all services
make stop
```

**Expected output:**
```
[+] Stopping containers...
 ✔ Container observability-ai-backend-1   Stopped
 ✔ Container observability-ai-postgres-1  Stopped
 ✔ Container observability-ai-redis-1     Stopped
```

**To completely clean up** (removes data, containers, and volumes):
```bash
make clean
```

**⚠️ Warning:** `make clean` will delete all your data including users, query history, and discovered services!

---

## Troubleshooting

Having issues? Here are solutions to common problems:

### Problem: "CLAUDE_API_KEY not found"

**Symptom:** Backend fails to start with error about missing API key

**Solution:**
```bash
# Check if .env exists
ls -la .env

# Verify API key is set
grep CLAUDE_API_KEY .env

# If missing, add it:
echo "CLAUDE_API_KEY=sk-ant-your-actual-key" >> .env

# Restart services
make restart
```

---

### Problem: "Port already in use"

**Symptom:** Error: `bind: address already in use`

**Solution:**
```bash
# Check what's using the ports
lsof -i :8080   # Backend
lsof -i :3000   # Frontend
lsof -i :5433   # PostgreSQL
lsof -i :6379   # Redis

# Kill the process using the port (example for 8080)
kill -9 $(lsof -t -i:8080)

# Or use different ports in .env:
# PORT=8081
# DB_PORT=5434
```

---

### Problem: "PostgreSQL not ready"

**Symptom:** Backend can't connect to database

**Solution:**
```bash
# Wait 10-15 seconds for PostgreSQL to fully start
sleep 15

# Check PostgreSQL health
make health-check

# If still not ready, check logs:
docker logs observability-ai-postgres-1

# Restart PostgreSQL
docker-compose restart postgres
```

---

### Problem: "Connection refused" to Claude API

**Symptom:** Queries fail with "connection refused" or "authentication failed"

**Solution:**
```bash
# Verify your API key is valid
curl https://api.anthropic.com/v1/complete \
  -H "x-api-key: $CLAUDE_API_KEY" \
  -H "anthropic-version: 2023-06-01"

# Check you have credits: https://console.anthropic.com/

# Update to correct API key in .env
```

---

### Problem: "401 Unauthorized" on queries

**Symptom:** API returns 401 when making queries

**Solution:**
```bash
# Your JWT token may have expired (24h lifetime)
# Login again to get a new token:
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "testuser", "password": "securepassword123"}'

# Store the new token
TOKEN="new-token-here"
```

---

### Problem: Frontend can't connect to backend

**Symptom:** Frontend shows connection errors

**Solution:**
```bash
# Verify backend is running
curl http://localhost:8080/health

# Check frontend API configuration
cat web/src/utils/api.ts
# Should point to: http://localhost:8080

# Check for CORS issues in browser console
# Backend should allow localhost:3000 origin
```

---

### Problem: "No services found"

**Symptom:** `/services` endpoint returns empty array

**Solution:**
```bash
# Check if service discovery is enabled
grep DISCOVERY_ENABLED .env
# Should be: DISCOVERY_ENABLED=true

# Verify Prometheus/Mimir endpoint is configured
grep MIMIR_URL .env
# Should be: MIMIR_URL=http://your-prometheus:9090

# Manually trigger discovery (as admin)
curl -X POST http://localhost:8080/admin/discovery/trigger \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Or load test data:
make test-db
```

---

### Problem: Queries are slow

**Symptom:** Queries take >5 seconds

**Possible causes:**
1. **First query after startup** - Semantic mapping is loading (normal)
2. **Claude API latency** - Check https://status.anthropic.com/
3. **Prometheus slow** - Check your Prometheus/Mimir performance
4. **No cache** - Subsequent identical queries should be faster

**Solution:**
```bash
# Check query execution time breakdown in response
# Look for "execution_time_ms" field

# Enable debug logging
export GIN_MODE=debug

# Check Redis is running (for cache)
docker ps | grep redis
```

---

### Still Having Issues?

1. **Check logs:**
   ```bash
   make logs
   # Or specific container:
   docker logs observability-ai-backend-1
   ```

2. **Check component health:**
   ```bash
   curl http://localhost:8080/api/v1/health
   ```

3. **Review full troubleshooting guide:**
   - [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) (comprehensive troubleshooting)
   - [docs/DEBUGGING.md](docs/DEBUGGING.md) (advanced debugging)

4. **Ask for help:**
   - [GitHub Issues](https://github.com/seanankenbruck/observability-ai/issues)
   - [GitHub Discussions](https://github.com/seanankenbruck/observability-ai/discussions)

---

## Next Steps

### Learn More

Now that you have Observability AI running, explore these resources:

- 📖 **[README.md](README.md)** - Full documentation and feature overview
- 🎯 **[docs/QUERY_EXAMPLES.md](docs/QUERY_EXAMPLES.md)** - 30+ query examples organized by use case
- 🏗️ **[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)** - How everything works under the hood
- ⚙️ **[docs/CONFIGURATION.md](docs/CONFIGURATION.md)** - All environment variables explained
- 💡 **[docs/WHY_OBSERVABILITY_AI.md](docs/WHY_OBSERVABILITY_AI.md)** - Deep dive on the value proposition

### Configure for Your Environment

- **Connect to your Prometheus/Mimir:**
  ```bash
  # Edit .env
  MIMIR_URL=http://your-prometheus:9090
  DISCOVERY_ENABLED=true
  ```

- **Enable authentication:**
  ```bash
  AUTH_ENABLED=true
  JWT_SECRET=your-secure-secret-here
  ```

- **Configure rate limiting:**
  ```bash
  RATE_LIMIT_ENABLED=true
  RATE_LIMIT_REQUESTS=100
  RATE_LIMIT_WINDOW=1h
  ```

### Explore Features

Try these features:

1. **Service Discovery** - Automatically discover your services:
   ```bash
   curl -X POST http://localhost:8080/admin/discovery/trigger \
     -H "Authorization: Bearer $ADMIN_TOKEN"
   ```

2. **API Keys** - Create API keys for programmatic access:
   ```bash
   curl -X POST http://localhost:8080/admin/api-keys \
     -H "Authorization: Bearer $ADMIN_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"name": "My App", "rate_limit": 1000}'
   ```

3. **Query History** - Review past queries:
   ```bash
   curl http://localhost:8080/history \
     -H "Authorization: Bearer $TOKEN"
   ```

### Useful Make Commands

```bash
make help                 # See all available commands
make test-unit           # Run unit tests
make test-integration    # Run integration tests
make psql                # Connect to PostgreSQL
make redis-cli           # Connect to Redis
make db-services         # List services in database
make db-metrics          # List metrics in database
```

### Deploy to Production

Ready to deploy? See:
- [docs/DEPLOYMENT_DOCKER.md](docs/DEPLOYMENT_DOCKER.md) - Docker deployment
- [docs/DEPLOYMENT_KUBERNETES.md](docs/DEPLOYMENT_KUBERNETES.md) - Kubernetes with Helm
- [docs/SECURITY_BEST_PRACTICES.md](docs/SECURITY_BEST_PRACTICES.md) - Production security

---

## Quick Reference

### Common Commands

```bash
# Start everything
make start-dev-docker          # Docker (everything)
make dev                       # Local (backend + frontend)

# Stop
make stop                      # Stop services
make clean                     # Stop and remove data

# Health checks
curl http://localhost:8080/health
make health-check

# Authentication
curl -X POST http://localhost:8080/auth/login -d '{"username":"...","password":"..."}'

# Query
curl -X POST http://localhost:8080/query \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query": "your question here"}'
```

### Default Ports

| Service    | Port | URL |
|------------|------|-----|
| Backend    | 8080 | http://localhost:8080 |
| Frontend   | 3000 | http://localhost:3000 |
| PostgreSQL | 5433 | localhost:5433 |
| Redis      | 6379 | localhost:6379 |

### Default Credentials (test-db)

If you ran `make test-db`:
- **Username:** `admin`
- **Password:** Check output of `make test-db` command

---

## Feedback & Contributing

- 🐛 **Found a bug?** [Open an issue](https://github.com/seanankenbruck/observability-ai/issues)
- 💡 **Have an idea?** [Start a discussion](https://github.com/seanankenbruck/observability-ai/discussions)
- 🤝 **Want to contribute?** See [CONTRIBUTING.md](CONTRIBUTING.md)

---

**🎉 You're all set!** Start asking questions and let Observability AI handle the PromQL.
