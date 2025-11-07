# Prometheus Integration

This document describes how Prometheus is integrated into the Observability AI Docker deployment to provide metrics collection and service discovery.

## Overview

The Docker deployment includes a complete observability stack:

1. **Prometheus** - Scrapes metrics from services
2. **Node Exporter** - Provides sample system metrics (simulating multiple services)
3. **Mimir** - Long-term metrics storage
4. **Service Discovery** - Discovers services from Mimir metrics

## Architecture

```
┌─────────────────┐
│  node-exporter  │  Provides sample metrics
└────────┬────────┘
         │ scrape
         ▼
┌─────────────────┐
│   Prometheus    │  Scrapes and aggregates metrics
└────────┬────────┘
         │ remote_write
         ▼
┌─────────────────┐
│     Mimir       │  Long-term metric storage
└────────┬────────┘
         │ query
         ▼
┌─────────────────┐
│ Service Disco.  │  Discovers services from metric labels
└─────────────────┘
```

## Configuration

### Prometheus Configuration

Located in [prometheus.yml](../prometheus.yml):

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s
  external_labels:
    cluster: 'docker-demo'
    environment: 'development'

# Remote write to Mimir
remote_write:
  - url: http://mimir:9009/api/v1/push
    headers:
      X-Scope-OrgID: demo

# Scrape configurations
scrape_configs:
  # Demo services (using node-exporter as a metric source)
  - job_name: 'demo-app'
    static_configs:
      - targets: ['node-exporter:9100']
        labels:
          service: 'demo-app'
          team: 'backend'
          environment: 'production'

  # Additional services: auth-service, payment-service, user-service
  # All use node-exporter as the metrics source but are labeled differently
```

### Service Discovery Configuration

The query processor is configured to discover services from Mimir:

```yaml
# Environment variables in docker-compose.yml
DISCOVERY_ENABLED: true
DISCOVERY_INTERVAL: 5m
SERVICE_LABEL_NAMES: service,job,app,application
EXCLUDE_METRICS: go_.*,process_.*,promhttp_.*
```

**How it works:**

1. Prometheus scrapes metrics from node-exporter every 15 seconds
2. Each scrape target has unique service labels (demo-app, auth-service, etc.)
3. Prometheus writes metrics to Mimir with these labels preserved
4. Service Discovery queries Mimir for metric series
5. Services are identified by the `service` label in the metrics
6. Metrics are filtered (excluding internal Go/process metrics)
7. Services and their metrics are stored in PostgreSQL

## Services Provided

The default configuration provides these demo services:

| Service | Description | Metrics Count |
|---------|-------------|---------------|
| demo-app | Sample demo application | ~276 |
| auth-service | Authentication service | ~276 |
| payment-service | Payment processing | ~276 |
| user-service | User management | ~276 |
| prometheus | Prometheus itself | ~232 |
| mimir | Mimir metrics storage | ~509 |

All services (except prometheus and mimir) use node-exporter as the metric source, differentiated by labels.

## Access Points

When running the Docker stack:

- **Prometheus UI**: http://localhost:9090
- **Mimir API**: http://localhost:9009
- **Node Exporter**: http://localhost:9100

## Verification

### Check Prometheus Targets

```bash
# Check if Prometheus is scraping targets
curl http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | {job: .labels.job, health: .health}'
```

Expected output:
```json
{"job":"demo-app","health":"up"}
{"job":"auth-service","health":"up"}
{"job":"payment-service","health":"up"}
{"job":"user-service","health":"up"}
{"job":"prometheus","health":"up"}
{"job":"mimir","health":"up"}
```

### Check Metrics in Mimir

```bash
# Query metrics from Mimir
curl -s "http://localhost:9009/prometheus/api/v1/query?query=up" \
  -H "X-Scope-OrgID: demo" | jq '.data.result[] | .metric.service'
```

Expected output:
```json
"demo-app"
"auth-service"
"payment-service"
"user-service"
"prometheus"
"mimir"
```

### Check Service Discovery

```bash
# Check discovery logs
docker logs observability-ai-query-processor | grep -i discovery

# Expected output:
# Discovered 6 services
# Created new service: default/demo-app with 276 metrics
# Discovery cycle completed in 5.46s: 6 services, 977 metrics, 6 database updates
```

### Query Services API

```bash
# Register a user
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"test","email":"test@example.com","password":"password123"}' \
  | jq -r '.token')

# Get services
curl -s http://localhost:8080/api/v1/services \
  -H "Authorization: Bearer $TOKEN" \
  | jq '.[].name'
```

Expected output:
```
"auth-service"
"demo-app"
"mimir"
"payment-service"
"prometheus"
"user-service"
```

## Timing Considerations

- **Initial startup**: Services take 15-30 seconds to appear after stack startup
- **Prometheus scrape interval**: 15 seconds
- **Service discovery interval**: 5 minutes (can be changed with `DISCOVERY_INTERVAL`)
- **First discovery**: Runs immediately on query-processor startup

To trigger immediate discovery after stack startup:
```bash
docker restart observability-ai-query-processor
```

## Troubleshooting

### No Services Discovered

1. **Check Prometheus is running**:
   ```bash
   curl http://localhost:9090/-/healthy
   ```

2. **Check Prometheus targets**:
   ```bash
   curl http://localhost:9090/api/v1/targets
   ```
   All targets should show `"health":"up"`

3. **Check metrics in Mimir**:
   ```bash
   curl -s "http://localhost:9009/prometheus/api/v1/query?query=up" \
     -H "X-Scope-OrgID: demo"
   ```

4. **Check query-processor logs**:
   ```bash
   docker logs observability-ai-query-processor | tail -50
   ```
   Look for discovery cycle messages

5. **Manually trigger discovery**:
   ```bash
   docker restart observability-ai-query-processor
   ```

### Node Exporter Failed to Start

On macOS, you may see: "path / is mounted on / but it is not a shared or slave mount"

**Solution**: The docker-compose.yml already includes the fix:
```yaml
node-exporter:
  volumes:
    - /:/host:ro  # Use 'ro' instead of 'ro,rslave'
  pid: host
```

### Metrics Not Showing in Mimir

1. **Check Prometheus remote write**:
   ```bash
   curl http://localhost:9090/api/v1/status/config | jq '.data.yaml' | grep -A5 remote_write
   ```

2. **Check Mimir logs**:
   ```bash
   docker logs observability-ai-mimir | tail -50
   ```

## Customization

### Adding Real Services

To add your own services for monitoring:

1. Update [prometheus.yml](../prometheus.yml):
   ```yaml
   scrape_configs:
     - job_name: 'my-service'
       static_configs:
         - targets: ['my-service:8080']
           labels:
             service: 'my-service'
             team: 'my-team'
   ```

2. Ensure your service exposes metrics at `/metrics` endpoint

3. Restart Prometheus:
   ```bash
   docker-compose restart prometheus
   ```

4. Wait for discovery cycle or restart query-processor:
   ```bash
   docker restart observability-ai-query-processor
   ```

### Adjusting Discovery Interval

In [docker-compose.yml](../docker-compose.yml):

```yaml
environment:
  DISCOVERY_ENABLED: true
  DISCOVERY_INTERVAL: 1m  # Change from 5m to 1m for faster discovery
```

### Changing Metric Exclusions

```yaml
environment:
  EXCLUDE_METRICS: go_.*,process_.*,promhttp_.*,node_scrape_.*
```

## Production Considerations

For production deployments:

1. **Use real service endpoints** instead of node-exporter simulation
2. **Configure authentication** for Prometheus and Mimir
3. **Set up proper retention policies** in Mimir
4. **Use service discovery** (Kubernetes, Consul) instead of static configs
5. **Enable TLS** for all communication
6. **Configure persistent storage** for Prometheus and Mimir data
7. **Set resource limits** on containers

## References

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Mimir Documentation](https://grafana.com/docs/mimir/latest/)
- [Node Exporter Documentation](https://github.com/prometheus/node_exporter)
- [Remote Write Specification](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#remote_write)
