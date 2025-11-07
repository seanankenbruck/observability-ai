# Configuration Reference

Complete reference for all environment variables and configuration options in Observability AI.

---

## Table of Contents

- [Quick Start](#quick-start)
- [Database Configuration](#database-configuration)
- [Redis Configuration](#redis-configuration)
- [Claude API Configuration](#claude-api-configuration)
- [Server Configuration](#server-configuration)
- [Prometheus/Mimir Configuration](#prometheusmir-configuration)
- [Service Discovery Configuration](#service-discovery-configuration)
- [Authentication Configuration](#authentication-configuration)
- [Rate Limiting Configuration](#rate-limiting-configuration)
- [Logging Configuration](#logging-configuration)
- [Configuration Presets](#configuration-presets)
- [Configuration Validation](#configuration-validation)

---

## Quick Start

All configuration is done through environment variables. The easiest way to configure Observability AI is to create a `.env` file in the project root:

```bash
# Copy the example file
cp .env.example .env

# Edit with your values
nano .env
```

**Minimum required configuration:**
```bash
CLAUDE_API_KEY=sk-ant-api03-your-key-here
```

---

## Database Configuration

PostgreSQL database settings with pgvector extension for semantic search.

### `DB_HOST`

**Description:** PostgreSQL server hostname or IP address
**Type:** String
**Default:** `localhost`
**Required:** Yes
**Valid Values:** Any valid hostname or IP address

**When to Change:**
- Use `postgres` if running backend inside Docker and PostgreSQL in the same docker-compose network
- Use actual hostname/IP for external database servers
- Use `localhost` for local development

**Example:**
```bash
# Local development
DB_HOST=localhost

# Docker Compose
DB_HOST=postgres

# External database
DB_HOST=prod-db.example.com
```

---

### `DB_PORT`

**Description:** PostgreSQL server port
**Type:** Integer
**Default:** `5433`
**Required:** Yes
**Valid Values:** 1-65535

**When to Change:**
- Default is `5433` to avoid conflicts with local PostgreSQL instances (which use 5432)
- Use `5432` if running backend inside Docker
- Match your actual PostgreSQL port for external databases

**Example:**
```bash
# Local development (avoid conflicts)
DB_PORT=5433

# Docker Compose
DB_PORT=5432

# Custom port
DB_PORT=15432
```

---

### `DB_NAME`

**Description:** PostgreSQL database name
**Type:** String
**Default:** `observability_ai`
**Required:** Yes
**Valid Values:** Valid PostgreSQL database name (alphanumeric, underscores)

**When to Change:**
- Use different names for different environments (dev, staging, prod)
- Use project-specific naming conventions

**Example:**
```bash
# Default
DB_NAME=observability_ai

# Environment-specific
DB_NAME=obs_ai_production
DB_NAME=obs_ai_staging
```

---

### `DB_USER`

**Description:** PostgreSQL username for authentication
**Type:** String
**Default:** `obs_ai`
**Required:** Yes
**Valid Values:** Valid PostgreSQL username

**Security:** Never use default usernames in production!

**Example:**
```bash
# Default (development only)
DB_USER=obs_ai

# Production (use unique username)
DB_USER=obs_ai_prod_user
```

---

### `DB_PASSWORD`

**Description:** PostgreSQL password for authentication
**Type:** String
**Default:** `changeme`
**Required:** Yes
**Valid Values:** Any string

**Security:**
- ⚠️ **CRITICAL:** Change the default password immediately!
- Use strong passwords (16+ characters, mixed case, numbers, symbols)
- Never commit passwords to version control
- Use secrets management in production (Vault, AWS Secrets Manager, etc.)

**Example:**
```bash
# ❌ NEVER use in production
DB_PASSWORD=changeme

# ✅ Strong password
DB_PASSWORD=xK9$mP2#vL8@qR5!wN3^
```

---

### `DB_SSLMODE`

**Description:** PostgreSQL SSL/TLS connection mode
**Type:** String
**Default:** `disable`
**Required:** No
**Valid Values:** `disable`, `allow`, `prefer`, `require`, `verify-ca`, `verify-full`

**When to Change:**
- Use `require` or `verify-full` for production
- Use `disable` only for local development

**Mode Descriptions:**
- `disable`: No SSL (fast, insecure)
- `allow`: Try non-SSL first, then SSL
- `prefer`: Try SSL first, then non-SSL
- `require`: SSL required, no cert verification
- `verify-ca`: SSL required, verify CA
- `verify-full`: SSL required, verify CA and hostname

**Example:**
```bash
# Development
DB_SSLMODE=disable

# Production (recommended)
DB_SSLMODE=verify-full
```

---

## Redis Configuration

Redis settings for caching, sessions, and rate limiting.

### `REDIS_ADDR`

**Description:** Redis server address (host:port format)
**Type:** String
**Default:** `localhost:6379`
**Required:** Yes
**Valid Values:** `host:port` format

**When to Change:**
- Use `redis:6379` if running inside Docker
- Match your Redis server address for external instances

**Example:**
```bash
# Local development
REDIS_ADDR=localhost:6379

# Docker Compose
REDIS_ADDR=redis:6379

# External Redis
REDIS_ADDR=redis.example.com:6379

# Redis Cluster (use sentinel or cluster client)
REDIS_ADDR=redis-cluster:6379
```

---

### `REDIS_PASSWORD`

**Description:** Redis authentication password
**Type:** String
**Default:** `changeme`
**Required:** No (but recommended)
**Valid Values:** Any string

**Security:**
- Change default password in production
- Leave empty only if Redis has no password configured
- Use strong passwords for production

**Example:**
```bash
# With password (recommended)
REDIS_PASSWORD=strong-redis-password-here

# No password (development only)
REDIS_PASSWORD=
```

---

### `REDIS_DB`

**Description:** Redis database number
**Type:** Integer
**Default:** `0`
**Required:** No
**Valid Values:** 0-15 (default Redis config)

**When to Change:**
- Use different DB numbers to separate environments on same Redis instance
- Useful for development/testing

**Example:**
```bash
# Default
REDIS_DB=0

# Separate by environment
REDIS_DB=1  # staging
REDIS_DB=2  # testing
```

---

## Claude API Configuration

Anthropic Claude API settings for PromQL generation.

### `CLAUDE_API_KEY`

**Description:** Anthropic Claude API key
**Type:** String
**Default:** None
**Required:** ✅ **YES** (critical)
**Valid Values:** Valid Claude API key starting with `sk-ant-`

**How to Get:**
1. Go to https://console.anthropic.com/
2. Create account or sign in
3. Navigate to API Keys
4. Generate new key

**Security:**
- ⚠️ **NEVER commit to version control**
- ⚠️ **NEVER expose in client-side code**
- Use secrets management in production
- Rotate keys periodically

**Example:**
```bash
CLAUDE_API_KEY=sk-ant-api03-...your-actual-key-here...
```

---

### `CLAUDE_MODEL`

**Description:** Claude model to use for query generation
**Type:** String
**Default:** `claude-3-haiku-20240307`
**Required:** No
**Valid Values:** Valid Claude model names or aliases

**Available Models:**

| Model | Speed | Cost | Accuracy | Best For |
|-------|-------|------|----------|----------|
| `claude-3-haiku-20240307` | ⚡⚡⚡ Fast | 💰 Low | ⭐⭐⭐ Good | Development, high-volume |
| `claude-3-sonnet-20240229` | ⚡⚡ Medium | 💰💰 Medium | ⭐⭐⭐⭐ Better | Production balance |
| `claude-3-opus-20240229` | ⚡ Slower | 💰💰💰 High | ⭐⭐⭐⭐⭐ Best | Complex queries, max accuracy |

**When to Change:**
- Start with Haiku for development
- Use Sonnet for production (good balance)
- Use Opus for complex query generation or maximum accuracy

**Cost Considerations:**
- Haiku: ~$0.001/query
- Sonnet: ~$0.003/query
- Opus: ~$0.015/query

**Example:**
```bash
# Fast and cheap (default)
CLAUDE_MODEL=claude-3-haiku-20240307

# Balanced (recommended for production)
CLAUDE_MODEL=claude-3-sonnet-20240229

# Maximum accuracy
CLAUDE_MODEL=claude-3-opus-20240229
```

---

### `CLAUDE_API_TIMEOUT`

**Description:** Timeout for Claude API requests (seconds)
**Type:** Integer
**Default:** `30`
**Required:** No
**Valid Values:** 1-120

**When to Change:**
- Increase if using Opus model (slower)
- Decrease for faster failure detection
- Increase for complex queries

**Example:**
```bash
# Default
CLAUDE_API_TIMEOUT=30

# For Opus model
CLAUDE_API_TIMEOUT=60

# Fast failure
CLAUDE_API_TIMEOUT=15
```

---

## Server Configuration

HTTP server and application settings.

### `PORT`

**Description:** HTTP server port
**Type:** Integer
**Default:** `8080`
**Required:** No
**Valid Values:** 1-65535

**When to Change:**
- Avoid conflicts with other services
- Match your load balancer/ingress configuration
- Use standard ports (80/443) behind reverse proxy

**Example:**
```bash
# Default
PORT=8080

# Alternative
PORT=3000

# Behind reverse proxy (requires root or capabilities)
PORT=80
```

---

### `GIN_MODE`

**Description:** Gin framework mode (affects logging and performance)
**Type:** String
**Default:** `debug`
**Required:** No
**Valid Values:** `debug`, `release`, `test`

**Mode Descriptions:**
- `debug`: Verbose logging, route debugging, slower
- `release`: Minimal logging, optimized performance
- `test`: Used by test suite

**When to Change:**
- **Always use `release` in production** for performance
- Use `debug` for local development
- Never use `test` manually

**Example:**
```bash
# Development
GIN_MODE=debug

# Production (required!)
GIN_MODE=release
```

---

### `LOG_LEVEL`

**Description:** Application log level
**Type:** String
**Default:** `info`
**Required:** No
**Valid Values:** `debug`, `info`, `warn`, `error`

**Level Descriptions:**
- `debug`: Everything (very verbose)
- `info`: Informational messages + warnings + errors
- `warn`: Warnings + errors only
- `error`: Errors only

**When to Change:**
- Use `debug` for troubleshooting
- Use `info` for development
- Use `warn` or `error` for production

**Example:**
```bash
# Development
LOG_LEVEL=info

# Production
LOG_LEVEL=warn

# Troubleshooting
LOG_LEVEL=debug
```

---

## Prometheus/Mimir Configuration

Settings for connecting to your metrics backend.

### `MIMIR_URL`

**Description:** Prometheus/Mimir server URL
**Type:** String (URL)
**Default:** `http://localhost:9009`
**Required:** Yes (for service discovery and query execution)
**Valid Values:** Valid HTTP/HTTPS URL

**When to Change:**
- Point to your actual Prometheus/Mimir instance
- Required for the application to function

**Example:**
```bash
# Local Prometheus
MIMIR_URL=http://localhost:9090

# Local Mimir
MIMIR_URL=http://localhost:9009

# Remote Prometheus
MIMIR_URL=https://prometheus.example.com

# Grafana Cloud Mimir
MIMIR_URL=https://prometheus-prod-us-central1.grafana.net
```

---

### `MIMIR_AUTH_TYPE`

**Description:** Authentication type for Prometheus/Mimir
**Type:** String
**Default:** `none`
**Required:** No
**Valid Values:** `none`, `basic`, `bearer`

**When to Change:**
- Use `basic` for username/password auth
- Use `bearer` for token-based auth
- Use `none` for unauthenticated (local development)

**Example:**
```bash
# No authentication (local)
MIMIR_AUTH_TYPE=none

# Basic auth
MIMIR_AUTH_TYPE=basic

# Bearer token (Grafana Cloud, etc.)
MIMIR_AUTH_TYPE=bearer
```

---

### `MIMIR_USERNAME` & `MIMIR_PASSWORD`

**Description:** Basic authentication credentials
**Type:** String
**Default:** Empty
**Required:** Only if `MIMIR_AUTH_TYPE=basic`
**Valid Values:** Any string

**Example:**
```bash
MIMIR_AUTH_TYPE=basic
MIMIR_USERNAME=admin
MIMIR_PASSWORD=your-prometheus-password
```

---

### `MIMIR_BEARER_TOKEN`

**Description:** Bearer token for authentication
**Type:** String
**Default:** Empty
**Required:** Only if `MIMIR_AUTH_TYPE=bearer`
**Valid Values:** Valid authentication token

**Example:**
```bash
MIMIR_AUTH_TYPE=bearer
MIMIR_BEARER_TOKEN=your-bearer-token-here
```

---

### `MIMIR_TENANT_ID`

**Description:** Mimir tenant/organization ID (X-Scope-OrgID header)
**Type:** String
**Default:** `demo`
**Required:** For multi-tenant Mimir deployments
**Valid Values:** Your tenant ID

**When to Change:**
- Required for Grafana Cloud Mimir
- Required for multi-tenant Mimir deployments
- Not needed for Prometheus

**Example:**
```bash
# Grafana Cloud
MIMIR_TENANT_ID=your-org-id

# Multi-tenant Mimir
MIMIR_TENANT_ID=production

# Single-tenant or Prometheus
MIMIR_TENANT_ID=
```

---

## Service Discovery Configuration

Automatic service and metric discovery settings.

### `DISCOVERY_ENABLED`

**Description:** Enable automatic service discovery
**Type:** Boolean
**Default:** `true`
**Required:** No
**Valid Values:** `true`, `false`

**When to Enable:**
- ✅ When you want automatic service/metric discovery
- ✅ When connected to Prometheus/Mimir with metrics

**When to Disable:**
- ❌ When manually managing services
- ❌ When Prometheus/Mimir is not available
- ❌ During testing without metrics backend

**Example:**
```bash
# Enable (recommended)
DISCOVERY_ENABLED=true

# Disable
DISCOVERY_ENABLED=false
```

---

### `DISCOVERY_INTERVAL`

**Description:** How often to run service discovery
**Type:** Duration string
**Default:** `5m`
**Required:** No
**Valid Values:** Duration format: `30s`, `5m`, `1h`, etc.

**Format:**
- `s`: seconds
- `m`: minutes
- `h`: hours

**When to Change:**
- Increase interval to reduce load on Prometheus/Mimir
- Decrease for faster detection of new services (increases load)

**Recommendations:**
- Development: `1m` (fast iteration)
- Production: `5m` (balanced)
- Large environments: `15m` (reduce load)

**Example:**
```bash
# Fast (development)
DISCOVERY_INTERVAL=1m

# Default (recommended)
DISCOVERY_INTERVAL=5m

# Slow (large environments)
DISCOVERY_INTERVAL=15m
```

---

### `DISCOVERY_NAMESPACES`

**Description:** Comma-separated list of Kubernetes namespaces to discover
**Type:** String (comma-separated)
**Default:** `default,production,staging`
**Required:** No
**Valid Values:** Comma-separated namespace names, or empty for all

**When to Change:**
- Limit discovery to specific namespaces
- Leave empty to discover all namespaces

**Example:**
```bash
# Specific namespaces
DISCOVERY_NAMESPACES=default,production,staging

# All namespaces
DISCOVERY_NAMESPACES=

# Single namespace
DISCOVERY_NAMESPACES=production
```

---

### `SERVICE_LABEL_NAMES`

**Description:** Comma-separated list of label names to identify services
**Type:** String (comma-separated)
**Default:** `service,job,app,application`
**Required:** No
**Valid Values:** Label names used in your metrics

**When to Change:**
- Match your metric labeling conventions
- Add custom label names your organization uses

**Example:**
```bash
# Default
SERVICE_LABEL_NAMES=service,job,app,application

# Custom
SERVICE_LABEL_NAMES=service,app,component,microservice
```

---

### `EXCLUDE_METRICS`

**Description:** Comma-separated regex patterns for metrics to exclude from discovery
**Type:** String (comma-separated regex)
**Default:** `go_.*,process_.*,promhttp_.*`
**Required:** No
**Valid Values:** Valid regex patterns

**When to Change:**
- Exclude noisy or irrelevant metrics
- Reduce database size
- Focus on application metrics

**Example:**
```bash
# Default (exclude Go runtime metrics)
EXCLUDE_METRICS=go_.*,process_.*,promhttp_.*

# Also exclude Kubernetes system metrics
EXCLUDE_METRICS=go_.*,process_.*,promhttp_.*,kube_.*,apiserver_.*

# Include everything
EXCLUDE_METRICS=
```

---

## Authentication Configuration

JWT and API key authentication settings.

### `JWT_SECRET`

**Description:** Secret key for signing JWT tokens
**Type:** String
**Default:** `your-secret-key-change-in-production`
**Required:** Yes
**Valid Values:** Any string (32+ characters recommended)

**Security:**
- ⚠️ **CRITICAL:** Change immediately!
- Use cryptographically random string
- Never commit to version control
- Generate with: `openssl rand -hex 32`

**Example:**
```bash
# ❌ NEVER use default
JWT_SECRET=your-secret-key-change-in-production

# ✅ Generate secure key
JWT_SECRET=$(openssl rand -hex 32)

# ✅ Example secure key
JWT_SECRET=a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x4y5z6
```

---

### `JWT_EXPIRY`

**Description:** JWT token expiration duration
**Type:** Duration string
**Default:** `24h`
**Required:** No
**Valid Values:** Duration format

**When to Change:**
- Shorter for higher security (users re-login more often)
- Longer for better UX (less re-authentication)

**Recommendations:**
- Development: `24h` (convenient)
- Production: `8h` to `24h` (balanced)
- High-security: `1h` to `4h`

**Example:**
```bash
# Default (24 hours)
JWT_EXPIRY=24h

# Short (high security)
JWT_EXPIRY=1h

# Long (convenience)
JWT_EXPIRY=168h  # 7 days
```

---

### `SESSION_EXPIRY`

**Description:** Session expiration in Redis
**Type:** Duration string
**Default:** `168h` (7 days)
**Required:** No
**Valid Values:** Duration format

**Example:**
```bash
# Default (7 days)
SESSION_EXPIRY=168h

# Short
SESSION_EXPIRY=24h
```

---

### `ALLOW_ANONYMOUS`

**Description:** Allow unauthenticated requests
**Type:** Boolean
**Default:** `false`
**Required:** No
**Valid Values:** `true`, `false`

**When to Enable:**
- ⚠️ Only for development/testing
- ⚠️ **NEVER enable in production**

**Security:**
- Disables all authentication checks
- Allows anyone to access the API

**Example:**
```bash
# Production (required!)
ALLOW_ANONYMOUS=false

# Development only
ALLOW_ANONYMOUS=true
```

---

## Rate Limiting Configuration

API rate limiting settings.

### `RATE_LIMIT`

**Description:** Default rate limit (requests per minute per client)
**Type:** Integer
**Default:** `100`
**Required:** No
**Valid Values:** Positive integer

**When to Change:**
- Increase for high-traffic APIs
- Decrease to protect against abuse
- Adjust based on Claude API quota

**Recommendations:**
- Development: `1000` (no limits)
- Production: `100` (reasonable)
- Public API: `50` (conservative)

**Example:**
```bash
# Default
RATE_LIMIT=100

# High traffic
RATE_LIMIT=500

# Conservative
RATE_LIMIT=50
```

---

## Configuration Presets

Ready-to-use configuration templates.

### Development (Local)

```bash
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
CLAUDE_API_KEY=sk-ant-api03-your-key-here
CLAUDE_MODEL=claude-3-haiku-20240307

# Server
PORT=8080
GIN_MODE=debug
LOG_LEVEL=info

# Mimir
MIMIR_URL=http://localhost:9009
MIMIR_AUTH_TYPE=none

# Discovery
DISCOVERY_ENABLED=true
DISCOVERY_INTERVAL=1m

# Auth
JWT_SECRET=dev-secret-not-for-production
JWT_EXPIRY=24h
ALLOW_ANONYMOUS=false

# Rate Limiting
RATE_LIMIT=1000
```

---

### Production (Docker Compose)

```bash
# Database
DB_HOST=postgres
DB_PORT=5432
DB_NAME=observability_ai
DB_USER=obs_ai_prod
DB_PASSWORD=<use-secrets-manager>
DB_SSLMODE=require

# Redis
REDIS_ADDR=redis:6379
REDIS_PASSWORD=<use-secrets-manager>

# Claude API
CLAUDE_API_KEY=<use-secrets-manager>
CLAUDE_MODEL=claude-3-sonnet-20240229

# Server
PORT=8080
GIN_MODE=release
LOG_LEVEL=warn

# Mimir
MIMIR_URL=https://prometheus.example.com
MIMIR_AUTH_TYPE=bearer
MIMIR_BEARER_TOKEN=<use-secrets-manager>

# Discovery
DISCOVERY_ENABLED=true
DISCOVERY_INTERVAL=5m
DISCOVERY_NAMESPACES=production
EXCLUDE_METRICS=go_.*,process_.*,promhttp_.*

# Auth
JWT_SECRET=<use-secrets-manager>
JWT_EXPIRY=8h
ALLOW_ANONYMOUS=false

# Rate Limiting
RATE_LIMIT=100
```

---

### Production (Kubernetes)

Use Kubernetes Secrets and ConfigMaps instead of .env file.

**ConfigMap:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: observability-ai-config
data:
  PORT: "8080"
  GIN_MODE: "release"
  LOG_LEVEL: "warn"
  DB_HOST: "postgres-service"
  DB_PORT: "5432"
  DB_NAME: "observability_ai"
  REDIS_ADDR: "redis-service:6379"
  CLAUDE_MODEL: "claude-3-sonnet-20240229"
  DISCOVERY_ENABLED: "true"
  DISCOVERY_INTERVAL: "5m"
  RATE_LIMIT: "100"
```

**Secret:**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: observability-ai-secrets
type: Opaque
stringData:
  DB_USER: "obs_ai_prod"
  DB_PASSWORD: "strong-password-here"
  CLAUDE_API_KEY: "sk-ant-api03-..."
  JWT_SECRET: "secure-random-string"
  REDIS_PASSWORD: "redis-password"
  MIMIR_BEARER_TOKEN: "bearer-token-here"
```

---

## Configuration Validation

### Validate Your Configuration

Use this checklist before deploying:

**Required Settings:**
- [ ] `CLAUDE_API_KEY` is set and valid
- [ ] `DB_*` settings point to accessible database
- [ ] `REDIS_ADDR` points to accessible Redis
- [ ] `MIMIR_URL` points to accessible Prometheus/Mimir

**Security Checklist:**
- [ ] Changed all default passwords
- [ ] `JWT_SECRET` is random and strong (32+ characters)
- [ ] `GIN_MODE=release` in production
- [ ] `DB_SSLMODE=require` or `verify-full` in production
- [ ] `ALLOW_ANONYMOUS=false` in production
- [ ] Secrets are not in version control
- [ ] Using secrets manager in production

**Performance Checklist:**
- [ ] `CLAUDE_MODEL` matches your accuracy/cost requirements
- [ ] `RATE_LIMIT` is appropriate for your traffic
- [ ] `DISCOVERY_INTERVAL` is reasonable for your environment
- [ ] `EXCLUDE_METRICS` filters unnecessary metrics

### Test Your Configuration

```bash
# Test database connection
make health-check

# Test full application
curl http://localhost:8080/health

# Check for configuration errors in logs
make logs
```

---

## Environment-Specific Configuration

### Load Configuration by Environment

```bash
# Development
cp .env.development .env

# Staging
cp .env.staging .env

# Production (use secrets manager instead!)
# DON'T use .env files in production
```

### Configuration Precedence

Environment variables are loaded in this order (later overrides earlier):

1. Built-in defaults (in code)
2. `.env` file
3. System environment variables
4. Command-line flags (if applicable)

**Example:**
```bash
# .env file sets:
PORT=8080

# Override with environment variable:
PORT=9090 ./bin/observability-ai

# Result: Application runs on port 9090
```

---

## Troubleshooting Configuration

### Common Issues

**Problem: "CLAUDE_API_KEY not found"**
```bash
# Check if set
echo $CLAUDE_API_KEY

# Check .env file
grep CLAUDE_API_KEY .env

# Set it
export CLAUDE_API_KEY=sk-ant-...
```

**Problem: "Database connection refused"**
```bash
# Check connection
pg_isready -h $DB_HOST -p $DB_PORT

# Test credentials
psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME
```

**Problem: "Redis connection failed"**
```bash
# Test connection
redis-cli -h localhost -p 6379 -a $REDIS_PASSWORD ping

# Should return: PONG
```

---

## Further Reading

- **[QUICKSTART.md](../QUICKSTART.md)** - Getting started guide
- **[TROUBLESHOOTING.md](TROUBLESHOOTING.md)** - Troubleshooting guide
- **[ARCHITECTURE.md](ARCHITECTURE.md)** - System architecture
- **[SECURITY_BEST_PRACTICES.md](SECURITY_BEST_PRACTICES.md)** - Production security

---

**Questions?** Open an issue on [GitHub](https://github.com/seanankenbruck/observability-ai/issues)
