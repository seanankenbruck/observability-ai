# Secret Management Guide

This guide shows you how to configure secrets for the Observability AI Query Processor in different deployment environments.

## Quick Start

The application supports three methods of secret management (in priority order):

1. **Kubernetes Secrets** (Production) - Auto-detected when running in K8s
2. **File-based Secrets** (Flexible) - Read from `/var/secrets` directory
3. **Environment Variables** (Development) - Traditional `.env` file approach

## Local Development

### Using Environment Variables (.env file)

This is the easiest method for local development:

```bash
# 1. Copy the example env file
cp .env.example .env

# 2. Edit with your values
vim .env

# 3. Run the application
go run cmd/query-processor/main.go
```

The application automatically falls back to environment variables when other methods aren't available.

## Docker / Docker Compose

### Option 1: Environment Variables (Simple)

```yaml
# docker-compose.yml
services:
  query-processor:
    image: observability-ai:latest
    environment:
      CLAUDE_API_KEY: ${CLAUDE_API_KEY}
      JWT_SECRET: ${JWT_SECRET}
      DB_PASSWORD: ${DB_PASSWORD}
      REDIS_PASSWORD: ${REDIS_PASSWORD}
```

```bash
# Set secrets in your shell or CI/CD
export CLAUDE_API_KEY="sk-ant-your-key"
export JWT_SECRET="your-secure-secret-at-least-32-chars"
export DB_PASSWORD="secure-password"
export REDIS_PASSWORD="secure-password"

docker-compose up
```

### Option 2: Docker Secrets (Recommended)

```yaml
# docker-compose.yml
version: '3.8'
services:
  query-processor:
    image: observability-ai:latest
    secrets:
      - claude_api_key
      - jwt_secret
      - db_password
      - redis_password
    environment:
      CLAUDE_API_KEY_FILE: /run/secrets/claude_api_key
      JWT_SECRET_FILE: /run/secrets/jwt_secret
      DB_PASSWORD_FILE: /run/secrets/db_password
      REDIS_PASSWORD_FILE: /run/secrets/redis_password

secrets:
  claude_api_key:
    file: ./secrets/claude_api_key.txt
  jwt_secret:
    file: ./secrets/jwt_secret.txt
  db_password:
    file: ./secrets/db_password.txt
  redis_password:
    file: ./secrets/redis_password.txt
```

## Kubernetes Deployment

### Method 1: Kubernetes Native Secrets

#### Step 1: Create Secret

```bash
# Create secret from literal values
kubectl create secret generic observability-ai-secrets \
  --from-literal=claude-api-key='sk-ant-your-key' \
  --from-literal=jwt-secret='your-secure-secret-at-least-32-chars' \
  --from-literal=db-password='secure-db-password' \
  --from-literal=redis-password='secure-redis-password'

# Or from files (recommended for security)
kubectl create secret generic observability-ai-secrets \
  --from-file=claude-api-key=./claude-key.txt \
  --from-file=jwt-secret=./jwt-secret.txt \
  --from-file=db-password=./db-password.txt \
  --from-file=redis-password=./redis-password.txt
```

#### Step 2: Mount Secret in Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: observability-ai
spec:
  template:
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

The application automatically detects it's running in Kubernetes and reads secrets from `/var/secrets`.

### Method 2: External Secrets Operator + Vault

This method integrates with HashiCorp Vault (or other secret backends).

#### Step 1: Install External Secrets Operator

```bash
helm repo add external-secrets https://charts.external-secrets.io
helm install external-secrets external-secrets/external-secrets
```

#### Step 2: Configure SecretStore

```yaml
apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: vault-backend
spec:
  provider:
    vault:
      server: "https://vault.example.com"
      path: "secret"
      version: "v2"
      auth:
        kubernetes:
          mountPath: "kubernetes"
          role: "observability-ai"
```

#### Step 3: Create ExternalSecret

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
  data:
    - secretKey: claude_api_key
      remoteRef:
        key: observability-ai
        property: claude_api_key
    - secretKey: jwt_secret
      remoteRef:
        key: observability-ai
        property: jwt_secret
    - secretKey: db_password
      remoteRef:
        key: observability-ai
        property: db_password
    - secretKey: redis_password
      remoteRef:
        key: observability-ai
        property: redis_password
```

#### Step 4: Store Secrets in Vault

```bash
vault kv put secret/observability-ai \
  claude_api_key="sk-ant-your-key" \
  jwt_secret="your-secure-secret-at-least-32-chars" \
  db_password="secure-db-password" \
  redis_password="secure-redis-password"
```

The External Secrets Operator will sync these into a Kubernetes secret, which the app reads automatically.

### Method 3: AWS Secrets Manager

#### Step 1: Store Secrets in AWS

```bash
aws secretsmanager create-secret \
  --name observability-ai/claude-api-key \
  --secret-string "sk-ant-your-key"

aws secretsmanager create-secret \
  --name observability-ai/jwt-secret \
  --secret-string "your-secure-secret-at-least-32-chars"

aws secretsmanager create-secret \
  --name observability-ai/db-password \
  --secret-string "secure-db-password"

aws secretsmanager create-secret \
  --name observability-ai/redis-password \
  --secret-string "secure-redis-password"
```

#### Step 2: Configure SecretStore

```yaml
apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: aws-secrets
spec:
  provider:
    aws:
      service: SecretsManager
      region: us-east-1
      auth:
        jwt:
          serviceAccountRef:
            name: observability-ai
```

#### Step 3: Create ExternalSecret

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: observability-ai-secrets
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: aws-secrets
    kind: SecretStore
  target:
    name: observability-ai-secrets
  data:
    - secretKey: claude-api-key
      remoteRef:
        key: observability-ai/claude-api-key
    - secretKey: jwt-secret
      remoteRef:
        key: observability-ai/jwt-secret
    - secretKey: db-password
      remoteRef:
        key: observability-ai/db-password
    - secretKey: redis-password
      remoteRef:
        key: observability-ai/redis-password
```

### Method 4: Azure Key Vault

Similar to AWS, using External Secrets Operator:

```yaml
apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: azure-keyvault
spec:
  provider:
    azurekv:
      vaultUrl: "https://my-vault.vault.azure.net"
      authType: ManagedIdentity
```

## Pulumi Deployment

### Using Pulumi Secrets

```typescript
import * as pulumi from "@pulumi/pulumi";
import * as k8s from "@pulumi/kubernetes";

// Store secrets in Pulumi config (encrypted)
const config = new pulumi.Config();
const claudeApiKey = config.requireSecret("claudeApiKey");
const jwtSecret = config.requireSecret("jwtSecret");
const dbPassword = config.requireSecret("dbPassword");
const redisPassword = config.requireSecret("redisPassword");

// Create Kubernetes secret
const secret = new k8s.core.v1.Secret("observability-ai-secrets", {
    metadata: {
        name: "observability-ai-secrets",
    },
    type: "Opaque",
    stringData: {
        "claude-api-key": claudeApiKey,
        "jwt-secret": jwtSecret,
        "db-password": dbPassword,
        "redis-password": redisPassword,
    },
});

// Deploy application with secret mount
const deployment = new k8s.apps.v1.Deployment("observability-ai", {
    spec: {
        template: {
            spec: {
                containers: [{
                    name: "query-processor",
                    image: "observability-ai:latest",
                    volumeMounts: [{
                        name: "secrets",
                        mountPath: "/var/secrets",
                        readOnly: true,
                    }],
                }],
                volumes: [{
                    name: "secrets",
                    secret: {
                        secretName: secret.metadata.name,
                    },
                }],
            },
        },
    },
});
```

Set secrets via Pulumi CLI:

```bash
pulumi config set --secret claudeApiKey sk-ant-your-key
pulumi config set --secret jwtSecret your-secure-secret-at-least-32-chars
pulumi config set --secret dbPassword secure-db-password
pulumi config set --secret redisPassword secure-redis-password
```

## Production Validation

When running in production mode (`GIN_MODE=release`), the application validates:

- ✅ No default passwords (e.g., `changeme`)
- ✅ JWT secret is at least 32 characters
- ✅ No placeholder API keys (e.g., `your-api-key-here`)
- ✅ Release mode is enabled
- ✅ Anonymous access is disabled
- ✅ Safety checks are enabled

If any of these checks fail, the application will refuse to start.

## Testing Your Setup

### Verify Configuration Loading

```bash
# Set log level to debug to see provider chain
export LOG_LEVEL=debug
go run cmd/query-processor/main.go

# Look for these log messages:
# - "Configuration loaded successfully from provider chain"
# - "Running in PRODUCTION mode" (if GIN_MODE=release)
```

### Verify Secret Sources

The application logs which provider succeeded:

```
2025/01/11 10:00:00 Configuration loaded successfully from provider chain
```

You can add debug logging in [internal/config/config.go](../internal/config/config.go) to see which provider was used.

### Test Production Validation

```bash
# This should FAIL with validation errors
export GIN_MODE=release
export JWT_SECRET=short
go run cmd/query-processor/main.go

# Expected error:
# Configuration validation failed: production validation failed:
# 1 validation error(s):
#   1. validation error: Auth.JWTSecret - JWT secret should be at least 32 characters for production use
```

## Troubleshooting

### "Configuration validation failed: Claude.APIKey is required"

Your Claude API key isn't being loaded. Check:
1. Is the secret set in your chosen method?
2. If using files, does `/var/secrets/claude-api-key` exist?
3. If using env vars, is `CLAUDE_API_KEY` set?

### "Production validation failed: DB_PASSWORD - must not use default password"

You're using `changeme` as the database password in production mode. Set a secure password.

### "Secret not found" in Kubernetes

1. Verify secret exists: `kubectl get secret observability-ai-secrets`
2. Check secret contents: `kubectl describe secret observability-ai-secrets`
3. Verify mount in pod: `kubectl exec <pod> -- ls -la /var/secrets`

### File permissions issue

Ensure secret files are readable:
```bash
kubectl exec <pod> -- ls -la /var/secrets
# Should show: -rw-r--r-- for each file
```

## Security Best Practices

1. **Never commit secrets to version control**
2. **Use different secrets for each environment** (dev, staging, production)
3. **Rotate secrets regularly** (at least quarterly)
4. **Use least-privilege access** - only give access to secrets that are needed
5. **Audit secret access** - monitor who accessed secrets and when
6. **Use short-lived secrets when possible** - consider dynamic secrets from Vault
7. **Encrypt secrets at rest** - use encrypted storage for secret backends

## Additional Resources

- [Configuration Package Documentation](../internal/config/README.md)
- [External Secrets Operator](https://external-secrets.io/)
- [HashiCorp Vault](https://www.vaultproject.io/)
- [AWS Secrets Manager](https://aws.amazon.com/secrets-manager/)
- [Azure Key Vault](https://azure.microsoft.com/en-us/services/key-vault/)
