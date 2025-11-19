# Observability AI Helm Chart

A Kubernetes Helm chart for deploying the Observability AI solution - an intelligent natural language interface for querying metrics from Prometheus/Mimir.

## Overview

This Helm chart deploys a complete Observability AI stack including:
- **Query Processor**: Backend API service that processes natural language queries using Claude AI
- **Web UI**: Frontend interface for users to interact with the system
- **Database Migrations**: Pre-install job to set up PostgreSQL schema
- **Optional Dependencies**: PostgreSQL and Redis can be deployed as subcharts or use external instances

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- PV provisioner support in the underlying infrastructure (for persistent storage)
- A Claude API key from Anthropic
- Access to a Prometheus/Mimir metrics backend

## Quick Start

### Development Installation

For local development with all dependencies included:

```bash
# Add Bitnami repository for PostgreSQL and Redis
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

# Install with development values
helm install observability-ai ./helm/observability-ai \
  -f ./helm/observability-ai/values-dev.yaml \
  --set secrets.claudeApiKey="your-claude-api-key" \
  --set config.mimir.endpoint="http://your-mimir:9009"
```

### Production Installation

For production with external managed databases:

```bash
# 1. Create a Kubernetes secret with production credentials
kubectl create secret generic observability-ai-prod-secrets \
  --from-literal=claude-api-key="sk-ant-your-real-key" \
  --from-literal=jwt-secret="your-super-secure-jwt-secret-at-least-32-characters" \
  --from-literal=db-password="secure-database-password" \
  --from-literal=redis-password="secure-redis-password"

# 2. Install with production values
helm install observability-ai ./helm/observability-ai \
  -f ./helm/observability-ai/values-prod.yaml \
  --set config.database.external.host="prod-postgres.example.com" \
  --set config.redis.external.host="prod-redis.example.com" \
  --set config.mimir.endpoint="http://mimir.monitoring:9009"
```

## Configuration

### Core Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `nameOverride` | Override the chart name | `""` |
| `fullnameOverride` | Override the full name | `""` |
| `namespace.create` | Create a namespace | `false` |
| `namespace.name` | Namespace name | `""` |

### Query Processor Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `queryProcessor.enabled` | Enable query processor deployment | `true` |
| `queryProcessor.replicaCount` | Number of replicas | `2` |
| `queryProcessor.image.repository` | Image repository | `ghcr.io/seanankenbruck/observability-ai-query-processor` |
| `queryProcessor.image.tag` | Image tag | Chart appVersion |
| `queryProcessor.service.type` | Service type | `ClusterIP` |
| `queryProcessor.service.port` | Service port | `8080` |
| `queryProcessor.resources.limits.cpu` | CPU limit | `1000m` |
| `queryProcessor.resources.limits.memory` | Memory limit | `512Mi` |
| `queryProcessor.autoscaling.enabled` | Enable HPA | `false` |

### Web UI Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `web.enabled` | Enable web UI deployment | `true` |
| `web.replicaCount` | Number of replicas | `2` |
| `web.image.repository` | Image repository | `ghcr.io/seanankenbruck/observability-ai-web` |
| `web.image.tag` | Image tag | Chart appVersion |
| `web.service.type` | Service type | `ClusterIP` |
| `web.service.port` | Service port | `3000` |

### Secret Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `secrets.existingSecret` | Use existing secret | `""` |
| `secrets.claudeApiKey` | Claude API key (inline) | `""` |
| `secrets.jwtSecret` | JWT secret (inline) | `""` |
| `secrets.dbPassword` | Database password (inline) | `""` |
| `secrets.redisPassword` | Redis password (inline) | `""` |

**Important**: For production, always use `existingSecret` and never commit inline secrets to version control.

### Application Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `config.server.ginMode` | Gin mode (debug/release/test) | `release` |
| `config.claude.model` | Claude model to use | `claude-3-haiku-20240307` |
| `config.mimir.endpoint` | Mimir/Prometheus endpoint | `http://mimir:9009` |
| `config.mimir.authType` | Mimir auth type (none/basic/bearer) | `none` |
| `config.auth.allowAnonymous` | Allow anonymous access | `false` |
| `config.query.enableSafetyChecks` | Enable query safety checks | `true` |

### Database Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `postgresql.enabled` | Deploy PostgreSQL subchart | `true` |
| `postgresql.auth.username` | PostgreSQL username | `obs_ai` |
| `postgresql.auth.password` | PostgreSQL password | `changeme` |
| `postgresql.auth.database` | Database name | `observability_ai` |
| `config.database.external.host` | External DB host (if postgresql.enabled=false) | `""` |
| `config.database.external.port` | External DB port | `5432` |
| `config.database.sslMode` | SSL mode | `disable` |

### Redis Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `redis.enabled` | Deploy Redis subchart | `true` |
| `redis.auth.password` | Redis password | `changeme` |
| `config.redis.external.host` | External Redis host (if redis.enabled=false) | `""` |
| `config.redis.external.port` | External Redis port | `6379` |

### Migration Job Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `migration.enabled` | Enable migration job | `true` |
| `migration.command` | Migration command | See values.yaml |
| `migration.backoffLimit` | Job retry limit | `3` |
| `migration.activeDeadlineSeconds` | Job timeout | `300` |

## Deployment Scenarios

### Scenario 1: Local Development

Use built-in PostgreSQL and Redis with reduced resources:

```bash
helm install observability-ai ./helm/observability-ai \
  -f ./helm/observability-ai/values-dev.yaml \
  --set secrets.claudeApiKey="sk-ant-dev-key"
```

### Scenario 2: Staging with External Databases

Use external managed databases but keep dev-like settings:

```bash
helm install observability-ai ./helm/observability-ai \
  -f ./helm/observability-ai/values-dev.yaml \
  --set postgresql.enabled=false \
  --set redis.enabled=false \
  --set config.database.external.host="staging-db.example.com" \
  --set config.redis.external.host="staging-redis.example.com" \
  --set secrets.claudeApiKey="sk-ant-staging-key"
```

### Scenario 3: Production with High Availability

Full production setup with autoscaling and external dependencies:

```bash
# Create production secret first
kubectl create secret generic observability-ai-prod-secrets \
  --from-literal=claude-api-key="sk-ant-prod-key" \
  --from-literal=jwt-secret="production-jwt-secret-at-least-32-characters" \
  --from-literal=db-password="prod-db-password" \
  --from-literal=redis-password="prod-redis-password"

# Install with production values
helm install observability-ai ./helm/observability-ai \
  -f ./helm/observability-ai/values-prod.yaml \
  --set config.database.external.host="prod-postgres.rds.amazonaws.com" \
  --set config.redis.external.host="prod-redis.elasticache.amazonaws.com" \
  --set config.mimir.endpoint="http://mimir.monitoring.svc.cluster.local:9009"
```

## Secret Management

### Option 1: Kubernetes Secrets (Basic)

Create a standard Kubernetes secret:

```bash
kubectl create secret generic my-secrets \
  --from-literal=claude-api-key="sk-ant-..." \
  --from-literal=jwt-secret="secure-secret-32-chars" \
  --from-literal=db-password="db-pass" \
  --from-literal=redis-password="redis-pass"

helm install observability-ai ./helm/observability-ai \
  --set secrets.existingSecret="my-secrets"
```

### Option 2: External Secrets Operator (Recommended)

Integrate with Vault, AWS Secrets Manager, or Azure Key Vault:

```yaml
# external-secret.yaml
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

Then install:

```bash
kubectl apply -f external-secret.yaml

helm install observability-ai ./helm/observability-ai \
  --set secrets.existingSecret="observability-ai-secrets"
```

### Option 3: Sealed Secrets

For GitOps workflows with encrypted secrets in Git:

```bash
# Encrypt your secret
kubectl create secret generic observability-ai-secrets \
  --from-literal=claude-api-key="sk-ant-..." \
  --dry-run=client -o yaml | \
  kubeseal -o yaml > sealed-secret.yaml

# Commit sealed-secret.yaml to Git
git add sealed-secret.yaml
git commit -m "Add sealed secrets"
```

## Accessing the Application

### Port Forwarding (Development)

```bash
# Access the Web UI
kubectl port-forward svc/observability-ai-web 3000:3000

# Access the API directly
kubectl port-forward svc/observability-ai-query-processor 8080:8080
```

### Ingress (Production)

Create an Ingress resource (not included in this chart):

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: observability-ai-ingress
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - observability.example.com
    secretName: observability-ai-tls
  rules:
  - host: observability.example.com
    http:
      paths:
      - path: /api
        pathType: Prefix
        backend:
          service:
            name: observability-ai-query-processor
            port:
              number: 8080
      - path: /
        pathType: Prefix
        backend:
          service:
            name: observability-ai-web
            port:
              number: 3000
```

## Monitoring and Operations

### Check Deployment Status

```bash
# Check all resources
helm status observability-ai

# Check pods
kubectl get pods -l app.kubernetes.io/instance=observability-ai

# Check services
kubectl get svc -l app.kubernetes.io/instance=observability-ai
```

### View Logs

```bash
# Query processor logs
kubectl logs -l app.kubernetes.io/component=query-processor -f

# Web UI logs
kubectl logs -l app.kubernetes.io/component=web -f

# Migration job logs
kubectl logs job/observability-ai-migrate
```

### Scaling

```bash
# Manual scaling
kubectl scale deployment observability-ai-query-processor --replicas=5

# Or use Helm upgrade
helm upgrade observability-ai ./helm/observability-ai \
  --set queryProcessor.replicaCount=5
```

### Check Autoscaling

```bash
# View HPA status
kubectl get hpa

# Describe HPA for details
kubectl describe hpa observability-ai-query-processor
```

## Upgrading

### Standard Upgrade

```bash
# Upgrade to new version
helm upgrade observability-ai ./helm/observability-ai \
  -f ./helm/observability-ai/values-prod.yaml
```

### Database Migrations

Database migrations run automatically as a pre-upgrade hook. The migration job will:
1. Run before the new deployment starts
2. Complete successfully before pods are updated
3. Fail the upgrade if migrations fail

Check migration status during upgrade:

```bash
kubectl get job observability-ai-migrate
kubectl logs job/observability-ai-migrate -f
```

## Uninstalling

```bash
# Uninstall the release
helm uninstall observability-ai

# Clean up PVCs (if needed)
kubectl delete pvc -l app.kubernetes.io/instance=observability-ai
```

## Troubleshooting

### Pods Not Starting

Check pod status and events:

```bash
kubectl get pods
kubectl describe pod <pod-name>
```

Common issues:
- **ImagePullBackOff**: Check image repository and imagePullSecrets
- **CrashLoopBackOff**: Check logs with `kubectl logs <pod-name>`
- **Pending**: Check resource quotas and PVC binding

### Secret Not Found

Verify the secret exists and has required keys:

```bash
kubectl get secret observability-ai-secrets
kubectl describe secret observability-ai-secrets
```

Required keys: `claude-api-key`, `jwt-secret`, `db-password`, `redis-password`

### Database Connection Issues

Check database connectivity:

```bash
# If using subchart
kubectl get pods -l app.kubernetes.io/name=postgresql

# Test connection from query processor
kubectl exec -it <query-processor-pod> -- /bin/sh
# Inside pod: ping postgresql service
```

### Migration Job Failed

Check migration job logs:

```bash
kubectl logs job/observability-ai-migrate
kubectl describe job observability-ai-migrate
```

Delete failed job and retry:

```bash
kubectl delete job observability-ai-migrate
helm upgrade observability-ai ./helm/observability-ai
```

## Advanced Configuration

### Custom Values Structure

Create a custom values file for your environment:

```yaml
# my-custom-values.yaml
queryProcessor:
  replicaCount: 5
  resources:
    limits:
      cpu: 2000m
      memory: 2Gi

config:
  claude:
    model: "claude-3-opus-20240229"  # Use more powerful model

  query:
    maxResultSamples: 20
    maxTimeRangeDays: 30

secrets:
  existingSecret: "my-production-secrets"
```

Install with custom values:

```bash
helm install observability-ai ./helm/observability-ai \
  -f ./helm/observability-ai/values-prod.yaml \
  -f my-custom-values.yaml
```

### Using with Service Mesh

For Istio/Linkerd integration, add mesh annotations:

```yaml
queryProcessor:
  podAnnotations:
    sidecar.istio.io/inject: "true"

web:
  podAnnotations:
    sidecar.istio.io/inject: "true"
```

### Resource Quotas

Set pod resource quotas for cost control:

```yaml
queryProcessor:
  resources:
    limits:
      cpu: 2000m
      memory: 1Gi
      ephemeral-storage: 1Gi
    requests:
      cpu: 500m
      memory: 512Mi
```

## Contributing

For issues, questions, or contributions:
- GitHub: https://github.com/seanankenbruck/observability-ai
- Issues: https://github.com/seanankenbruck/observability-ai/issues

## License

See the main repository for license information.
