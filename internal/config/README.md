# Configuration Package

The `config` package provides a flexible, multi-source configuration system designed for cloud-native deployments, with first-class support for Kubernetes secret management.

## Features

- **Multi-Source Secret Providers**: Load configuration from Kubernetes secrets, file-based secrets, or environment variables
- **Priority Chain**: Automatic fallback from Kubernetes → File → Environment variables
- **Production Validation**: Comprehensive validation with additional security checks for production deployments
- **Type Safety**: Strongly-typed configuration with proper parsing for durations, integers, booleans, and slices
- **Cloud-Native Ready**: Designed for Helm charts and Kubernetes deployments

## Architecture

### Secret Providers

The package uses a provider chain pattern where secrets can be loaded from multiple sources:

```
┌─────────────────┐
│ K8s Provider    │ (Priority 1: Kubernetes secrets via file mounts)
└────────┬────────┘
         ↓ (if not found)
┌─────────────────┐
│ File Provider   │ (Priority 2: File-based secrets from /var/secrets)
└────────┬────────┘
         ↓ (if not found)
┌─────────────────┐
│ Env Provider    │ (Priority 3: Environment variables - fallback)
└─────────────────┘
```

### Provider Types

#### 1. Kubernetes Provider (`K8sProvider`)
- Reads secrets from Kubernetes-mounted files (default: `/var/run/secrets/...`)
- Auto-detects if running in a Kubernetes pod
- Falls back to `FileProvider` for actual file reading
- Future-ready for direct Kubernetes API integration

#### 2. File Provider (`FileProvider`)
- Reads secrets from individual files in a directory
- Default path: `/var/secrets`
- Follows Kubernetes secret mount conventions
- Converts environment variable names to file names (e.g., `CLAUDE_API_KEY` → `claude-api-key`)

#### 3. Environment Variable Provider (`EnvProvider`)
- Reads from standard environment variables
- Always available as the final fallback
- Used for local development and backward compatibility

#### 4. Chain Provider (`ChainProvider`)
- Orchestrates multiple providers with fallback logic
- Tries providers in order until one succeeds
- Used by `NewDefaultLoader()`

## Usage

### Basic Usage

```go
import (
    "context"
    "github.com/seanankenbruck/observability-ai/internal/config"
)

func main() {
    ctx := context.Background()

    // Create loader with default provider chain
    loader := config.NewDefaultLoader()

    // Load configuration
    cfg := loader.MustLoad(ctx)

    // Validate configuration
    if err := cfg.ValidateWithContext(); err != nil {
        log.Fatalf("Configuration validation failed: %v", err)
    }

    // Use configuration
    fmt.Printf("Server will listen on port: %s\n", cfg.Server.Port)
    fmt.Printf("Database host: %s\n", cfg.Database.Host)
}
```

### Custom Provider Chain

```go
// Create a custom provider chain
providers := []config.SecretProvider{
    config.NewFileProvider("/custom/secrets/path"),
    config.NewEnvProvider(),
}

loader := config.NewLoader(config.NewChainProvider(providers...))
cfg := loader.MustLoad(ctx)
```

### Environment Variable Only (Legacy)

```go
// Use only environment variables
loader := config.NewLoader(config.NewEnvProvider())
cfg := loader.MustLoad(ctx)
```

## Configuration Structure

The `Config` struct contains all application configuration organized by component:

```go
type Config struct {
    Database  DatabaseConfig   // PostgreSQL configuration
    Redis     RedisConfig      // Redis configuration
    Claude    ClaudeConfig     // Claude LLM API configuration
    Mimir     MimirConfig      // Mimir/Prometheus configuration
    Discovery DiscoveryConfig  // Service discovery configuration
    Auth      AuthConfig       // Authentication & authorization
    Server    ServerConfig     // HTTP server configuration
    Query     QueryConfig      // Query processing configuration
}
```

### Configuration Sections

#### Database Configuration
```go
DB_HOST=localhost
DB_PORT=5432
DB_NAME=observability_ai
DB_USER=obs_ai
DB_PASSWORD=secure-password
DB_SSLMODE=disable
```

#### Redis Configuration
```go
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=secure-redis-password
REDIS_DB=0
```

#### Claude Configuration
```go
CLAUDE_API_KEY=sk-ant-your-api-key
CLAUDE_MODEL=claude-3-haiku-20240307
```

#### Mimir Configuration
```go
MIMIR_ENDPOINT=http://localhost:9009
MIMIR_AUTH_TYPE=none          # Options: none, basic, bearer
MIMIR_USERNAME=               # Required if auth_type=basic
MIMIR_PASSWORD=               # Required if auth_type=basic
MIMIR_BEARER_TOKEN=           # Required if auth_type=bearer
MIMIR_TENANT_ID=demo
MIMIR_TIMEOUT=30s
```

#### Authentication Configuration
```go
JWT_SECRET=your-secure-jwt-secret-at-least-32-chars
JWT_EXPIRY=24h
SESSION_EXPIRY=168h           # 7 days
RATE_LIMIT=100                # requests per minute per client
ALLOW_ANONYMOUS=false
```

#### Server Configuration
```go
PORT=8080
GIN_MODE=debug                # Options: debug, release, test
```

#### Query Configuration
```go
MAX_RESULT_SAMPLES=10
MAX_RESULT_TIMEPOINTS=50
QUERY_TIMEOUT=30s
CACHE_TTL=5m
MAX_QUERY_LENGTH=500
MAX_NESTING_DEPTH=3
MAX_TIME_RANGE_DAYS=7
ENABLE_SAFETY_CHECKS=true
FORBIDDEN_METRIC_NAMES=.*_secret.*,.*_password.*,.*_token.*
```

## Validation

### Basic Validation

All configurations undergo basic validation to ensure required fields are present and values are within acceptable ranges:

```go
cfg, _ := loader.Load(ctx)

// Validate configuration
if err := cfg.Validate(); err != nil {
    // Handle validation errors
    log.Fatalf("Config validation failed: %v", err)
}
```

### Production Validation

When `GIN_MODE=release`, additional security checks are enforced:

```go
if err := cfg.ValidateWithContext(); err != nil {
    // This runs both basic AND production validation
    log.Fatalf("Production validation failed: %v", err)
}
```

Production validation checks for:
- Default or insecure passwords (e.g., `changeme`)
- Short JWT secrets (< 32 characters)
- Placeholder API keys
- Debug mode in production
- Anonymous access enabled
- Safety checks disabled

## Kubernetes Deployment

### Using Kubernetes Secrets

1. **Create a Kubernetes Secret:**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: observability-ai-secrets
type: Opaque
stringData:
  claude-api-key: "sk-ant-your-real-key"
  jwt-secret: "your-super-secure-jwt-secret-with-at-least-32-characters"
  db-password: "secure-database-password"
  redis-password: "secure-redis-password"
```

2. **Mount the Secret in Your Pod:**

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: observability-ai
spec:
  containers:
  - name: query-processor
    image: observability-ai:latest
    volumeMounts:
    - name: secrets
      mountPath: /var/secrets
      readOnly: true
  volumes:
  - name: secrets
    secret:
      secretName: observability-ai-secrets
```

3. **Application Auto-Discovery:**

The application will automatically detect it's running in Kubernetes and load secrets from `/var/secrets`. No code changes needed!

### Using External Secrets Operator

For integration with external secret stores (Vault, AWS Secrets Manager, etc.):

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: observability-ai-secrets
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: vault-backend
    kind: SecretStore
  target:
    name: observability-ai-secrets
    template:
      data:
        claude-api-key: "{{ .claude_api_key }}"
        jwt-secret: "{{ .jwt_secret }}"
        db-password: "{{ .db_password }}"
        redis-password: "{{ .redis_password }}"
  dataFrom:
    - extract:
        key: secret/data/observability-ai
```

The application remains unchanged - it just reads from `/var/secrets`.

## Helm Chart Integration

When creating a Helm chart, use values to control secret sources:

```yaml
# values.yaml
secrets:
  # Source: "kubernetes" or "external"
  source: kubernetes

  # For kubernetes source, provide inline values (dev only!)
  inline:
    claudeApiKey: ""
    jwtSecret: ""
    dbPassword: ""
    redisPassword: ""

  # For external source, reference existing secret
  existingSecret: "observability-ai-secrets"
```

Template example:
```yaml
# templates/secret.yaml
{{- if eq .Values.secrets.source "kubernetes" }}
apiVersion: v1
kind: Secret
metadata:
  name: {{ include "observability-ai.fullname" . }}-secrets
type: Opaque
stringData:
  claude-api-key: {{ .Values.secrets.inline.claudeApiKey | required "claudeApiKey is required" }}
  jwt-secret: {{ .Values.secrets.inline.jwtSecret | required "jwtSecret is required" }}
  db-password: {{ .Values.secrets.inline.dbPassword | required "dbPassword is required" }}
  redis-password: {{ .Values.secrets.inline.redisPassword | required "redisPassword is required" }}
{{- end }}
```

## Local Development

For local development, continue using `.env` files with environment variables:

```bash
# Copy example env file
cp .env.example .env

# Edit with your values
vim .env

# Run the application (uses EnvProvider as fallback)
go run cmd/query-processor/main.go
```

The configuration system will automatically fall back to environment variables when Kubernetes/file-based secrets aren't available.

## Migration from Environment Variables

The new config package is **backward compatible** with environment variable configuration. No changes required for existing deployments!

The migration path:
1. ✅ **Keep using env vars** - Works out of the box with `EnvProvider` fallback
2. **Add file-based secrets** - Mount secrets to `/var/secrets` for improved security
3. **Deploy to Kubernetes** - Auto-detects K8s and uses mounted secrets
4. **Integrate external provider** - Use External Secrets Operator with Vault/AWS/etc.

## Testing

### Running Tests

```bash
# Run all config tests
go test ./internal/config -v

# Run specific test
go test ./internal/config -v -run TestConfigValidation

# Run with coverage
go test ./internal/config -cover
```

### Example Test

```go
func TestConfigLoading(t *testing.T) {
    ctx := context.Background()

    // Set up test environment
    os.Setenv("DB_HOST", "test-db")
    defer os.Unsetenv("DB_HOST")

    // Load config
    loader := config.NewLoader(config.NewEnvProvider())
    cfg, err := loader.Load(ctx)

    if err != nil {
        t.Fatalf("failed to load config: %v", err)
    }

    if cfg.Database.Host != "test-db" {
        t.Errorf("expected host 'test-db', got '%s'", cfg.Database.Host)
    }
}
```

## Best Practices

1. **Use Default Loader**: `NewDefaultLoader()` provides the best experience across all environments
2. **Always Validate**: Call `cfg.ValidateWithContext()` to catch configuration issues early
3. **Production Secrets**: Never commit secrets to version control or use default values in production
4. **Secret Rotation**: Design for secret rotation - avoid caching secrets indefinitely
5. **Least Privilege**: Mount only the secrets your application needs
6. **Audit Logging**: Log configuration source (but never log secret values)

## Future Enhancements

- [ ] Direct Kubernetes API integration (alternative to file mounts)
- [ ] Support for AWS Secrets Manager SDK
- [ ] Support for HashiCorp Vault SDK
- [ ] Support for Azure Key Vault
- [ ] Configuration hot-reloading
- [ ] Encrypted configuration files
- [ ] Configuration versioning and rollback

## Troubleshooting

### Config validation fails with "JWT secret is required"

The JWT secret is empty. Check:
1. Is `JWT_SECRET` set in your environment?
2. Is the secret file mounted at `/var/secrets/jwt-secret`?
3. Do you have read permissions on the secret file?

### Production validation fails

You're running in `release` mode with insecure defaults. Either:
1. Set `GIN_MODE=debug` for development
2. Update all secrets to secure, non-default values

### Secrets not loading from Kubernetes

Check:
1. Secret is created: `kubectl get secret observability-ai-secrets`
2. Secret is mounted: `kubectl exec <pod> -- ls -la /var/secrets`
3. File permissions allow reading
4. Secret key names match expected format (kebab-case)

### File provider not available

The `/var/secrets` directory doesn't exist or isn't accessible. Options:
1. Create the directory
2. Use a custom path with `NewFileProvider("/custom/path")`
3. Fall back to environment variables only

## Support

For issues or questions about the configuration system:
- GitHub Issues: https://github.com/seanankenbruck/observability-ai/issues
- See also: [Production Readiness Guide](../../docs/PRODUCTION_READINESS.md)
