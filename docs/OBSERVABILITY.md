# Observability Features

## Overview

The Observability AI query processor includes comprehensive observability features to monitor, debug, and optimize the service itself. This document describes the structured logging, metrics collection, and health check capabilities.

## Features

### 1. Structured Logging with Correlation IDs

All logs are output in structured JSON format with correlation IDs for request tracing.

#### Log Format

```json
{
  "timestamp": "2025-01-07T10:30:45Z",
  "level": "info",
  "message": "Processing query",
  "correlation_id": "550e8400-e29b-41d4-a716-446655440000",
  "user_id": "user123",
  "component": "query-processor",
  "operation": "process_query",
  "duration_ms": 245,
  "fields": {
    "query": "Show error rate for api-gateway",
    "cache_hit": false
  }
}
```

#### Log Levels

- **debug**: Detailed debugging information
- **info**: General informational messages
- **warn**: Warning messages for non-critical issues
- **error**: Error messages for failures

#### Correlation IDs

Every HTTP request automatically receives a correlation ID that tracks it through the entire system:

- Automatically generated if not provided
- Included in the `X-Request-ID` header
- Propagated through all internal operations
- Included in all log entries

#### Usage

```go
logger := observability.NewLogger("my-component")

// Log with context
logger.Info(ctx, "Operation started", map[string]interface{}{
    "operation": "query_processing",
    "user_id": "user123",
})

// Log errors
logger.Error(ctx, "Operation failed", err, map[string]interface{}{
    "query": "invalid query",
})
```

### 2. Metrics Collection

The system tracks comprehensive metrics for all operations.

#### Available Metrics

**Query Processing Metrics:**
- `query_processor_queries_total` - Total number of queries processed
- `query_processor_query_duration_seconds` - Query processing latency
- `query_processor_queries_success_total` - Successful queries
- `query_processor_queries_failure_total` - Failed queries
- `query_processor_cache_hits_total` - Cache hit count
- `query_processor_cache_misses_total` - Cache miss count
- `query_processor_safety_violations_total` - Safety check violations

**LLM Metrics:**
- `llm_requests_total` - Total LLM API requests
- `llm_request_duration_seconds` - LLM request latency
- `llm_tokens_total` - Total tokens consumed
- `llm_cost_dollars` - Accumulated LLM costs
- `llm_errors_total` - LLM request failures
- `llm_embedding_requests_total` - Embedding generation requests

**Database Metrics:**
- `database_queries_total` - Total database queries
- `database_query_duration_seconds` - Query latency
- `database_errors_total` - Database errors
- `database_connections_active` - Active connections
- `database_connection_pool_size` - Connection pool size

**Authentication Metrics:**
- `auth_attempts_total` - Login attempts
- `auth_success_total` - Successful logins
- `auth_failure_total` - Failed logins
- `auth_tokens_created_total` - Tokens created
- `auth_sessions_active` - Active sessions
- `auth_apikey_requests_total` - API key requests

**HTTP Metrics:**
- `http_requests_total` - Total HTTP requests
- `http_request_duration_seconds` - Request latency
- `http_errors_total` - HTTP errors (4xx, 5xx)
- `http_response_size_bytes` - Response sizes

**Discovery Metrics:**
- `discovery_runs_total` - Discovery service runs
- `discovery_duration_seconds` - Discovery duration
- `discovery_services_found` - Services discovered
- `discovery_metrics_found` - Metrics discovered
- `discovery_errors_total` - Discovery errors

#### Metrics Endpoint

Access metrics at: `GET /metrics`

```json
{
  "metrics": {
    "query_processor_queries_total": {
      "name": "query_processor_queries_total",
      "type": "counter",
      "value": 1250,
      "timestamp": "2025-01-07T10:30:45Z"
    },
    "query_processor_query_duration_seconds": {
      "name": "query_processor_query_duration_seconds",
      "type": "histogram",
      "value": 0.245,
      "extra": {
        "count": 1250,
        "sum": 306.25
      }
    }
  },
  "timestamp": "2025-01-07T10:30:45Z"
}
```

#### Recording Metrics

```go
// Record query metrics
observability.RecordQueryMetrics(duration, success, cached, errorType)

// Record LLM metrics
observability.RecordLLMMetrics(operation, duration, tokens, cost, err)

// Record database metrics
observability.RecordDBMetrics(operation, duration, err)

// Record HTTP metrics
observability.RecordHTTPMetrics(method, path, statusCode, duration, responseSize)
```

### 3. Health Checks

Comprehensive health checks verify all dependencies are operational.

#### Health Endpoint

Access health status at: `GET /health`

```json
{
  "status": "healthy",
  "timestamp": "2025-01-07T10:30:45Z",
  "checks": {
    "database": {
      "name": "database",
      "status": "healthy",
      "message": "Database connection successful",
      "last_checked": "2025-01-07T10:30:45Z",
      "duration_ms": 5,
      "metadata": {
        "response_time_ms": 5
      }
    },
    "redis": {
      "name": "redis",
      "status": "healthy",
      "message": "Redis connection successful",
      "last_checked": "2025-01-07T10:30:45Z",
      "duration_ms": 2,
      "metadata": {
        "response_time_ms": 2
      }
    },
    "memory": {
      "name": "memory",
      "status": "healthy",
      "message": "Memory usage normal",
      "last_checked": "2025-01-07T10:30:45Z",
      "duration_ms": 0,
      "metadata": {
        "used_bytes": 134217728,
        "total_bytes": 1073741824,
        "usage_percent": 12.5
      }
    }
  },
  "metadata": {
    "version": "1.0.0",
    "service": "query-processor"
  }
}
```

#### Health Status Values

- **healthy**: All systems operational
- **degraded**: Some non-critical systems unavailable
- **unhealthy**: Critical systems unavailable

#### HTTP Status Codes

- `200 OK` - Service is healthy or degraded
- `503 Service Unavailable` - Service is unhealthy

#### Registered Health Checks

1. **Database**: Verifies PostgreSQL connectivity
2. **Redis**: Verifies Redis cache connectivity
3. **Memory**: Monitors memory usage
4. **LLM Service** (optional): Verifies AI service availability
5. **Mimir** (optional): Verifies Prometheus/Mimir connectivity

#### Custom Health Checks

```go
healthChecker := observability.NewHealthChecker()

// Register custom check
healthChecker.Register("my_service", func(ctx context.Context) *observability.HealthCheck {
    return &observability.HealthCheck{
        Name:    "my_service",
        Status:  observability.HealthStatusHealthy,
        Message: "Service is operational",
    }
})
```

### 4. Request Tracking Middleware

Automatic request tracking with correlation IDs and metrics.

#### Features

- **Correlation ID Propagation**: Automatically generates and propagates request IDs
- **Request Logging**: Logs all incoming requests with full context
- **Response Logging**: Logs response status, duration, and size
- **Metrics Recording**: Automatically records HTTP metrics
- **Error Handling**: Captures and logs request errors
- **Panic Recovery**: Recovers from panics with proper logging

#### Middleware Stack

```go
router.Use(observability.RecoveryMiddleware(logger))
router.Use(observability.RequestLoggingMiddleware(logger))
router.Use(observability.MetricsMiddleware())
```

#### Request Headers

- `X-Request-ID`: Correlation ID (auto-generated if not provided)
- Response includes `X-Request-ID` header

## Monitoring Best Practices

### 1. Structured Logging

- Always include correlation IDs in logs
- Use appropriate log levels
- Include relevant context in log fields
- Never log sensitive information (passwords, tokens)

### 2. Metrics

- Monitor query success rate (should be > 95%)
- Track query latency (p50, p95, p99)
- Monitor LLM costs and token usage
- Watch for safety violations
- Track cache hit rate (should be > 50% for production)

### 3. Health Checks

- Monitor health endpoint regularly (every 10-30 seconds)
- Alert on unhealthy status
- Track degraded status as warning
- Check dependency response times

### 4. Alerting Thresholds

**Critical:**
- Health status: unhealthy
- Query success rate < 90%
- Database connection failures
- Redis connection failures

**Warning:**
- Health status: degraded
- Query success rate < 95%
- Query latency p95 > 2s
- Cache hit rate < 40%
- Memory usage > 75%

## Integration Examples

### Prometheus Integration

The metrics format is compatible with Prometheus. Use a custom exporter or parse the JSON endpoint.

```yaml
scrape_configs:
  - job_name: 'observability-ai'
    scrape_interval: 30s
    metrics_path: '/metrics'
    static_configs:
      - targets: ['localhost:8080']
```

### Grafana Dashboards

Create dashboards tracking:
- Query processing rate and latency
- LLM costs over time
- Cache hit rates
- Error rates by type
- System health status

### Log Aggregation

Forward structured logs to:
- **Elasticsearch/Kibana**: For log analysis
- **Loki**: For lightweight log aggregation
- **CloudWatch/Stackdriver**: For cloud deployments

Example Fluentd configuration:

```
<source>
  @type tail
  path /var/log/query-processor/*.log
  pos_file /var/log/query-processor.pos
  tag observability.logs
  format json
</source>
```

### Correlation ID Tracing

Use correlation IDs to trace requests across:
1. API Gateway
2. Query Processor
3. LLM Service
4. Database
5. Cache

## Performance Impact

The observability features are designed for minimal performance impact:

- **Logging**: < 1ms overhead per log entry
- **Metrics**: < 0.1ms overhead per metric update
- **Health Checks**: Cached for 5 seconds by default
- **Middleware**: < 0.5ms overhead per request

## Environment Variables

Configure logging and observability:

```bash
# Log level (debug, info, warn, error)
LOG_LEVEL=info

# Enable structured logging
LOG_FORMAT=json

# Enable request logging
REQUEST_LOGGING=true

# Health check cache TTL
HEALTH_CHECK_TTL=5s
```

## Troubleshooting

### High Query Latency

1. Check `query_processor_query_duration_seconds` metrics
2. Review logs for slow operations
3. Check LLM response times
4. Verify database performance
5. Check cache hit rate

### Memory Issues

1. Monitor memory health check
2. Check for memory leaks in metrics
3. Review connection pool sizes
4. Check cache size limits

### Database Connection Issues

1. Check database health status
2. Review connection pool metrics
3. Check `database_errors_total` metric
4. Review database logs with correlation IDs

## Future Enhancements

1. **Distributed Tracing**: OpenTelemetry integration
2. **Custom Metrics**: User-defined business metrics
3. **Alert Manager**: Built-in alerting based on metrics
4. **SLO Tracking**: Service Level Objective monitoring
5. **Cost Analysis**: Detailed LLM cost breakdown by user/query type

## Conclusion

The observability features provide comprehensive insight into the query processor's operation, enabling:

- **Debugging**: Trace requests through the system
- **Performance Optimization**: Identify bottlenecks
- **Cost Management**: Track and optimize LLM costs
- **Reliability**: Monitor system health
- **User Experience**: Track success rates and latency
