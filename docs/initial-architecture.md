# Observability AI Service - Initial Architecture

## Technology Stack

### Core Services
- **Language**: Go (for performance, concurrency, and Kubernetes ecosystem fit)
- **Database**: PostgreSQL (for semantic mappings, query history, user sessions)
- **Cache**: Redis (for query results, semantic embeddings)
- **Message Queue**: Redis (for async query processing)

### AI/ML Components
- **Embedding Model**: OpenAI text-embedding-3-small or sentence-transformers
- **LLM**: OpenAI GPT-4 or Claude (configurable)
- **Vector Database**: pgvector (PostgreSQL extension) for semantic search

## Service Architecture

### 1. API Gateway Service (`api-gateway`)
```go
// Handles authentication, rate limiting, request routing
- HTTP REST API
- WebSocket for real-time chat
- Authentication middleware
- Request validation
- Rate limiting per user/organization
```

### 2. Query Processor Service (`query-processor`)
```go
// Core intelligence - converts natural language to PromQL
- Intent classification
- Named entity recognition
- PromQL query generation
- Query validation and safety checks
- Caching layer for similar queries
```

### 3. Semantic Mapping Service (`semantic-mapper`)
```go
// Manages service topology and metric definitions
- Auto-discovery of Mimir metrics
- Metric metadata storage
- Service dependency mapping
- Custom mapping configuration
```

### 4. Mimir Proxy Service (`mimir-proxy`)
```go
// Handles all Mimir interactions
- Connection pooling
- Query execution
- Result formatting
- Error handling and retries
- Query performance monitoring
```

### 5. Configuration Service (`config-service`)
```go
// Manages user configurations and service topology
- Service definitions
- Metric naming conventions
- User preferences
- Organization settings
```

### 6. Web Interface (`web-ui`)
```typescript
// React-based frontend
- Chat interface
- Configuration dashboards
- Query history
- Service topology visualization
```

## Data Models

### Core Entities
```sql
-- Services and their relationships
CREATE TABLE services (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    namespace VARCHAR(255),
    description TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Metric definitions and metadata
CREATE TABLE metrics (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    service_id UUID REFERENCES services(id),
    description TEXT,
    metric_type VARCHAR(50), -- counter, gauge, histogram
    labels JSONB,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Query history for learning
CREATE TABLE query_history (
    id UUID PRIMARY KEY,
    user_id VARCHAR(255),
    natural_query TEXT,
    generated_promql TEXT,
    success BOOLEAN,
    execution_time_ms INTEGER,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Semantic embeddings for similarity search
CREATE TABLE query_embeddings (
    id UUID PRIMARY KEY,
    query_text TEXT,
    embedding vector(1536), -- OpenAI embedding dimension
    promql_template TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);
```

## Helm Chart Structure

```yaml
# values.yaml structure
global:
  domain: "observability-ai.local"
  tls:
    enabled: true
    secretName: "observability-ai-tls"

# Service configurations
services:
  apiGateway:
    replicas: 2
    resources:
      requests:
        memory: "128Mi"
        cpu: "100m"
      limits:
        memory: "256Mi"
        cpu: "200m"

  queryProcessor:
    replicas: 3
    resources:
      requests:
        memory: "256Mi"
        cpu: "200m"
      limits:
        memory: "512Mi"
        cpu: "500m"

  semanticMapper:
    replicas: 2
    resources:
      requests:
        memory: "128Mi"
        cpu: "100m"
      limits:
        memory: "256Mi"
        cpu: "200m"

  mimirProxy:
    replicas: 2
    resources:
      requests:
        memory: "128Mi"
        cpu: "100m"
      limits:
        memory: "256Mi"
        cpu: "200m"

  configService:
    replicas: 1
    resources:
      requests:
        memory: "128Mi"
        cpu: "100m"
      limits:
        memory: "256Mi"
        cpu: "200m"

# External dependencies
postgresql:
  enabled: true
  auth:
    database: "observability_ai"
    username: "obs_ai"
    password: "changeme"
  primary:
    persistence:
      size: "20Gi"

redis:
  enabled: true
  auth:
    enabled: true
    password: "changeme"
  master:
    persistence:
      size: "8Gi"

# User configuration
config:
  # Mimir connection settings
  mimir:
    endpoint: "http://mimir-query-frontend:8080"
    auth:
      type: "basic" # basic, bearer, none
      username: ""
      password: ""

  # AI service configuration
  ai:
    provider: "openai" # openai, claude, local
    apiKey: ""
    model: "gpt-4"
    embeddingModel: "text-embedding-3-small"

  # Service discovery
  discovery:
    enabled: true
    interval: "5m"
    namespaces: ["default", "monitoring"]

  # Query safety
  safety:
    maxQueryRange: "7d"
    maxCardinality: 10000
    timeoutSeconds: 30
```

## Directory Structure

```
observability-ai/
├── cmd/
│   ├── api-gateway/
│   ├── query-processor/
│   ├── semantic-mapper/
│   ├── mimir-proxy/
│   └── config-service/
├── internal/
│   ├── auth/
│   ├── config/
│   ├── database/
│   ├── llm/
│   ├── mimir/
│   ├── promql/
│   └── semantic/
├── web/
│   ├── src/
│   ├── public/
│   └── package.json
├── helm/
│   ├── templates/
│   ├── values.yaml
│   └── Chart.yaml
├── migrations/
├── docker/
└── docs/
```

## Key Go Packages Structure

```go
// internal/llm/client.go
type LLMClient interface {
    GenerateQuery(ctx context.Context, prompt string) (*QueryResult, error)
    GetEmbedding(ctx context.Context, text string) ([]float32, error)
}

// internal/promql/generator.go
type QueryGenerator struct {
    semanticMapper *semantic.Mapper
    safety         *SafetyChecker
}

// internal/mimir/client.go
type Client struct {
    endpoint string
    auth     AuthConfig
    timeout  time.Duration
}

// internal/semantic/mapper.go
type Mapper struct {
    db    *sql.DB
    cache *redis.Client
}
```

## API Endpoints

```
POST /api/v1/query
GET  /api/v1/services
POST /api/v1/services
GET  /api/v1/metrics
POST /api/v1/config/discovery
GET  /api/v1/history
WS   /api/v1/chat
```

## Deployment Flow

1. **Prerequisites Check**: Verify Mimir connectivity
2. **Database Setup**: Run migrations, create extensions
3. **Service Discovery**: Auto-discover existing metrics
4. **Semantic Bootstrapping**: Generate initial embeddings
5. **Health Checks**: Verify all services are operational

## Security Considerations

- **Authentication**: Support for OIDC, basic auth, API keys
- **Authorization**: Role-based access control
- **Query Safety**: Prevent expensive queries, timeout protection
- **Data Privacy**: No sensitive data in logs, encryption at rest
- **Network Security**: TLS everywhere, network policies

## Monitoring & Observability

- **Metrics**: Prometheus metrics from all services
- **Logs**: Structured logging with correlation IDs
- **Tracing**: OpenTelemetry tracing for request flows
- **Health**: Kubernetes health and readiness probes
- **Alerting**: Key SLI monitoring (query success rate, latency)
