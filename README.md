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
- Automatic service discovery from Prometheus/Mimir
- API key authentication and management
- Rate limiting and usage tracking

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
CLAUDE_MODEL=claude-3-haiku-20240307 # Alias or API model name

# Server Configuration
PORT=8080
GIN_MODE=debug            # Use 'release' for production

# Service Discovery Configuration
DISCOVERY_ENABLED=true
DISCOVERY_INTERVAL=5m     # How often to discover services/metrics
MIMIR_URL=http://localhost:9009  # Your Prometheus/Mimir endpoint

# Authentication Configuration
AUTH_ENABLED=true         # Enable API key authentication
JWT_SECRET=your-secret-key-here

# Rate Limiting Configuration
RATE_LIMIT_ENABLED=true
RATE_LIMIT_REQUESTS=100   # Requests per window
RATE_LIMIT_WINDOW=1h      # Time window for rate limiting
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
│   └── test-db/            # Database test utility
├── internal/
│   ├── auth/               # Authentication handlers
│   ├── database/           # Reusable database utilities
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

### Public Endpoints
- `GET /health` - Global health check
- `GET /api/v1/health` - API endpoint health check
- `POST /auth/register` - Register new user
- `POST /auth/login` - Login and get JWT token

### Protected Endpoints (Require Authentication)
- `POST /query` - Process natural language query
- `GET /history` - Query history
- `GET /services` - List available services
- `GET /metrics` - List available metrics

### Admin Endpoints (Require Admin Role)
- `GET /admin/api-keys` - List all API keys
- `POST /admin/api-keys` - Create new API key
- `PUT /admin/api-keys/:id` - Update API key
- `DELETE /admin/api-keys/:id` - Delete API key
- `GET /admin/users/:id/usage` - Get user usage statistics
- `POST /admin/discovery/trigger` - Manually trigger service discovery

Example authenticated query:
```bash
# First, login to get a token
TOKEN=$(curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "your-password"}' \
  | jq -r '.token')

# Then use the token for API requests
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query": "What is the CPU usage for the auth service?"}'
```

## Service Discovery Setup

Observability AI can automatically discover services and metrics from your Prometheus/Mimir instance, eliminating the need for manual configuration.

### Configuration

Enable service discovery in your `.env` file:

```bash
DISCOVERY_ENABLED=true
DISCOVERY_INTERVAL=5m
MIMIR_URL=http://localhost:9009
```

### How It Works

1. **Automatic Discovery**: The system periodically queries your Prometheus/Mimir endpoint for all available metrics
2. **Service Extraction**: Services are identified from metric labels (typically `service`, `job`, or `app` labels)
3. **Metric Cataloging**: All discovered metrics are stored in the semantic database with their labels
4. **Semantic Mapping**: Metrics are automatically mapped for natural language queries

### Manual Trigger

You can manually trigger a discovery run:

```bash
# Using the admin API
curl -X POST http://localhost:8080/admin/discovery/trigger \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Or check discovery status
curl http://localhost:8080/admin/discovery/status \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Discovery Process

The discovery service:
- Runs on startup and then at configured intervals
- Queries Prometheus/Mimir for all time series metadata
- Extracts service names from label patterns
- Stores metrics and their relationships in PostgreSQL
- Updates semantic embeddings for improved query matching

### Monitored Services

View discovered services:

```bash
# List all discovered services
curl http://localhost:8080/services \
  -H "Authorization: Bearer $TOKEN"

# List all discovered metrics
curl http://localhost:8080/metrics \
  -H "Authorization: Bearer $TOKEN"
```

## Authentication Configuration

The system uses JWT-based authentication with role-based access control (RBAC).

### User Roles

- **User**: Can query metrics and view history
- **Admin**: Full system access including API key management and user administration

### Initial Setup

1. **Configure authentication** in `.env`:

```bash
AUTH_ENABLED=true
JWT_SECRET=your-secure-secret-key-here
```

2. **Create the admin user**:

```bash
# The system creates a default admin user on first run
# Username: admin
# Password: Check logs or set via environment variable
```

3. **Login to get JWT token**:

```bash
TOKEN=$(curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "your-password"}' \
  | jq -r '.token')
```

### User Registration

Register new users via the API:

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "new-user",
    "password": "secure-password",
    "email": "user@example.com"
  }'
```

### Token Usage

Include the JWT token in the `Authorization` header for all protected endpoints:

```bash
curl -X POST http://localhost:8080/query \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query": "show me CPU usage"}'
```

### Token Expiration

- Default token lifetime: 24 hours
- Refresh tokens before expiration by logging in again
- The system returns token expiration time in the login response

## API Key Management

Administrators can create API keys for programmatic access and service accounts.

### Creating API Keys

```bash
# Create a new API key
curl -X POST http://localhost:8080/admin/api-keys \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Production Service",
    "description": "API key for production monitoring service",
    "expires_at": "2025-12-31T23:59:59Z",
    "rate_limit": 1000
  }'
```

Response:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "key": "obs_ai_1234567890abcdef",
  "name": "Production Service",
  "created_at": "2025-01-15T10:00:00Z",
  "expires_at": "2025-12-31T23:59:59Z"
}
```

**Important**: Save the API key immediately - it's only shown once!

### Using API Keys

API keys can be used instead of JWT tokens:

```bash
curl -X POST http://localhost:8080/query \
  -H "X-API-Key: obs_ai_1234567890abcdef" \
  -H "Content-Type: application/json" \
  -d '{"query": "What is memory usage?"}'
```

### Managing API Keys

```bash
# List all API keys
curl http://localhost:8080/admin/api-keys \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Update an API key
curl -X PUT http://localhost:8080/admin/api-keys/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Updated Name",
    "rate_limit": 2000
  }'

# Revoke an API key
curl -X DELETE http://localhost:8080/admin/api-keys/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### API Key Features

- **Named keys**: Assign meaningful names for easy identification
- **Expiration dates**: Set automatic expiration for security
- **Per-key rate limits**: Configure different limits for different consumers
- **Usage tracking**: Monitor API key usage and statistics
- **Instant revocation**: Delete keys immediately when compromised

## Rate Limiting

Rate limiting prevents abuse and ensures fair resource allocation across users and API keys.

### Configuration

Configure rate limiting in `.env`:

```bash
RATE_LIMIT_ENABLED=true
RATE_LIMIT_REQUESTS=100
RATE_LIMIT_WINDOW=1h
```

### Rate Limit Tiers

Different rate limits apply based on authentication method:

| Authentication | Default Limit | Window | Notes |
|----------------|---------------|--------|-------|
| API Keys | Per-key config | 1 hour | Set during key creation |
| JWT (User) | 100 requests | 1 hour | Per user account |
| JWT (Admin) | 1000 requests | 1 hour | Elevated limits |

### Rate Limit Headers

All API responses include rate limit information:

```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1642348800
```

### Handling Rate Limits

When rate limited, the API returns HTTP 429:

```json
{
  "error": "rate limit exceeded",
  "retry_after": 3600,
  "limit": 100,
  "window": "1h"
}
```

### Usage Monitoring

Administrators can monitor usage:

```bash
# Get usage statistics for a user
curl http://localhost:8080/admin/users/USER_ID/usage \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

Response:
```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "total_requests": 1234,
  "current_window": {
    "requests": 45,
    "limit": 100,
    "remaining": 55,
    "reset_at": "2025-01-15T11:00:00Z"
  },
  "last_24h": 523,
  "last_7d": 2841
}
```

### Adjusting Rate Limits

Admins can adjust per-user or per-key limits:

```bash
# Update API key rate limit
curl -X PUT http://localhost:8080/admin/api-keys/KEY_ID \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"rate_limit": 5000}'
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
