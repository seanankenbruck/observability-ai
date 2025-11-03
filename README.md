# Observability AI

A natural language interface for querying Prometheus/Mimir metrics. Ask questions in plain English and get PromQL queries executed against your observability data.

## Overview

Observability AI converts natural language queries into PromQL using Claude AI, enabling intuitive exploration of your metrics without memorizing query syntax.

**Key Features:**
- Natural language to PromQL translation
- Semantic mapping of services and metrics
- Query caching and history
- Safety validation for queries
- React-based web UI

## Architecture

- **Backend**: Go (Gin) + PostgreSQL (pgvector) + Redis + Claude API
- **Frontend**: React + TypeScript + Tailwind CSS + Vite
- **Infrastructure**: Docker Compose / Kubernetes (Helm)

## Prerequisites

- **Docker** and **Docker Compose**
- **Go 1.21+** (for local development)
- **Node.js 18+** and **npm** (for frontend development)
- **Claude API Key** from [Anthropic](https://console.anthropic.com/)

## Quick Start

### Option 1: Full Docker Setup (Recommended for first-time users)

This starts everything (PostgreSQL, Redis, and the backend API) in Docker:

```bash
# 1. Set your Claude API key (required)
export CLAUDE_API_KEY="your-api-key-here"

# 2. Start all services with Docker
make start-dev-docker

# 3. Access the API
# Backend API: http://localhost:8080
# Health check: http://localhost:8080/health
```

**Note**: The Makefile references `deploy/configs/development.env` which doesn't exist yet. You'll need to create it or modify the `.env` file manually after `make setup`.

### Option 2: Local Development (Backend + Frontend)

This runs PostgreSQL/Redis in Docker but the backend and frontend locally:

```bash
# 1. Start PostgreSQL and Redis
make setup

# 2. Create .env file with your Claude API key
cat > .env << EOF
# Database
DB_HOST=localhost
DB_PORT=5433
DB_NAME=observability_ai
DB_USER=obs_ai
DB_PASSWORD=changeme
DB_SSLMODE=disable

# Redis
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=changeme

# Claude API
CLAUDE_API_KEY=your-api-key-here
CLAUDE_MODEL=claude-3-haiku-20240307

# Server
PORT=8080
GIN_MODE=debug
EOF

# 3. Run database migrations
make migrate

# 4. (Optional) Load test data
make test-db

# 5. Start the full development environment
make dev
```

This will:
- Start the backend on http://localhost:8080
- Start the frontend on http://localhost:3000 (with hot reload)

### Option 3: Backend Only

```bash
# 1. Setup and migrate (as above)
make setup migrate

# 2. Start just the backend
make start-backend
# Or: make run-query-processor
```

### Option 4: Frontend Only

```bash
# Start the frontend dev server
make start-frontend

# Or manually:
cd web
npm install
npm run dev
```

## Environment Variables

Create a `.env` file in the project root with these variables:

```bash
# Database Configuration
DB_HOST=localhost          # Use 'postgres' if running in Docker
DB_PORT=5433              # 5433 for local, 5432 for Docker
DB_NAME=observability_ai
DB_USER=obs_ai
DB_PASSWORD=changeme
DB_SSLMODE=disable

# Redis Configuration
REDIS_ADDR=localhost:6379  # Use 'redis:6379' if running in Docker
REDIS_PASSWORD=changeme

# Claude API Configuration (REQUIRED)
CLAUDE_API_KEY=sk-ant-...  # Get from https://console.anthropic.com/
CLAUDE_MODEL=claude-3-haiku-20240307

# Server Configuration
PORT=8080
GIN_MODE=debug            # Use 'release' for production
```

## Available Make Commands

### Development
- `make help` - Show all available commands
- `make dev` - Start full development environment (backend + frontend)
- `make start-backend` - Start only the backend Go server
- `make start-frontend` - Start only the frontend dev server
- `make start-dev-docker` - Start everything with Docker Compose

### Setup & Database
- `make setup` - Start PostgreSQL and Redis containers
- `make migrate` - Run database migrations
- `make test-db` - Load sample data into the database
- `make start` - Shortcut for `setup migrate test-db`

### Testing
- `make test-unit` - Run Go unit tests
- `make test-integration` - Run integration tests

### Building
- `make build` - Build the Go backend binary
- `make build-web` - Build the frontend for production
- `make serve` - Preview production build locally

### Database Utilities
- `make psql` - Connect to PostgreSQL with psql
- `make redis-cli` - Connect to Redis
- `make db-services` - List all services in the database
- `make db-metrics` - List all metrics in the database
- `make db-embeddings` - List query embeddings
- `make health-check` - Check PostgreSQL health

### Cleanup
- `make stop` - Stop all services
- `make restart` - Restart backend services
- `make clean` - Remove all containers and volumes
- `make logs` - Show Docker logs

## Project Structure

```
observability-ai/
├── cmd/
│   ├── query-processor/    # Main HTTP API server
│   ├── migrate/            # Database migration tool
│   ├── test-db/            # Database test utility
│   ├── test-integration/   # Integration tests
│   └── test-llm/           # LLM client tests
├── internal/
│   ├── llm/                # Claude API client
│   ├── processor/          # Query processing & safety
│   ├── semantic/           # Semantic mapping (PostgreSQL)
│   ├── mimir/              # Mimir/Prometheus client
│   ├── promql/             # PromQL utilities
│   ├── database/           # Database utilities
│   └── config/             # Configuration
├── web/
│   ├── src/
│   │   ├── components/     # React components
│   │   ├── types/          # TypeScript types
│   │   └── utils/          # API client
│   └── dist/               # Production build output
├── migrations/             # SQL migrations
├── helm/                   # Kubernetes charts
├── docker-compose.yml      # Full Docker setup
├── docker-compose.test.yml # Test environment
└── Makefile               # Development commands
```

## API Endpoints

Once running, the backend exposes:

- `GET /health` - Health check
- `POST /query` - Process natural language query
- `GET /history` - Query history
- `GET /services` - List available services
- `GET /metrics` - List available metrics

Example query:
```bash
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -d '{"query": "What is the CPU usage for the auth service?"}'
```

## Development Workflow

1. **Make code changes** to Go files in `internal/` or `cmd/`
2. **Restart backend**: Press Ctrl+C and run `make start-backend` again
3. **Frontend changes** are auto-reloaded by Vite

For database schema changes:
1. Create a new migration in `migrations/`
2. Run `make migrate`

## Troubleshooting

### "CLAUDE_API_KEY not found"
Make sure you've set the API key in your `.env` file or environment.

### "PostgreSQL is not ready"
Wait a few seconds for PostgreSQL to fully start, then try again.

### Port already in use
Check if services are already running:
```bash
lsof -i :8080   # Backend
lsof -i :3000   # Frontend
lsof -i :5433   # PostgreSQL
lsof -i :6379   # Redis
```

### Database connection refused
Ensure PostgreSQL is running:
```bash
make health-check
```

### Frontend can't connect to backend
Make sure the backend is running on port 8080 and check [web/src/utils/api.ts](web/src/utils/api.ts) for the correct API URL.

## Contributing

1. Make sure tests pass: `make test-unit`
2. Format code: `make fmt`
3. Run linter: `make lint`

## License

[Add your license here]
