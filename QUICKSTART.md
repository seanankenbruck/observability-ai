# Quick Start Guide

Get Observability AI running in under 5 minutes!

## Prerequisites

- Docker and Docker Compose installed
- A Claude API key from [Anthropic Console](https://console.anthropic.com/)

## 1. Clone and Configure

```bash
# Navigate to the project directory
cd observability-ai

# Copy the example environment file
cp .env.example .env

# Edit .env and add your Claude API key
# Replace 'your-api-key-here' with your actual API key
```

## 2. Choose Your Setup Method

### Method A: Local Development (Recommended)

Best for development with hot-reload:

```bash
# 1. Start PostgreSQL and Redis
make setup

# 2. Run migrations
make migrate

# 3. (Optional) Load test data
make test-db

# 4. Start backend + frontend
make dev
```

**Access:**
- Frontend: http://localhost:3000
- Backend API: http://localhost:8080
- Health Check: http://localhost:8080/health

### Method B: Full Docker Setup

Best for quick demos or if you don't want to install Go/Node:

```bash
# 1. Add your API key to deploy/configs/development.env
# Edit the file and set CLAUDE_API_KEY=your-key

# 2. Start everything with Docker
make start-dev-docker
```

**Access:**
- Backend API: http://localhost:8080
- Health Check: http://localhost:8080/health

## 3. Test the API

```bash
# Health check
curl http://localhost:8080/health

# Example natural language query
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -d '{"query": "What is the CPU usage for the auth service?"}'

# List services
curl http://localhost:8080/services

# List metrics
curl http://localhost:8080/metrics
```

## 4. Stop Everything

```bash
# Stop all services
make stop

# Or completely clean up (removes data)
make clean
```

## Troubleshooting

### "CLAUDE_API_KEY not found"

Make sure you've added your API key to `.env`:
```bash
echo "CLAUDE_API_KEY=sk-ant-your-key-here" >> .env
```

### "Port already in use"

Check what's running on the ports:
```bash
lsof -i :8080   # Backend
lsof -i :3000   # Frontend
lsof -i :5433   # PostgreSQL
```

### PostgreSQL not ready

Wait 10-15 seconds for PostgreSQL to initialize, then check:
```bash
make health-check
```

## Next Steps

- See [README.md](README.md) for full documentation
- Check `make help` for all available commands
- Explore the codebase structure in [README.md#project-structure](README.md#project-structure)
