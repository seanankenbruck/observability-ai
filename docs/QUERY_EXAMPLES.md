# Query Examples Library

This document shows real examples of natural language queries and the PromQL they generate. Use these as inspiration for your own queries!

---

## Table of Contents

- [Resource Metrics](#resource-metrics)
  - [CPU Usage](#cpu-usage)
  - [Memory Usage](#memory-usage)
  - [Disk I/O](#disk-io)
  - [Network Traffic](#network-traffic)
- [Application Metrics](#application-metrics)
  - [Request Rates](#request-rates)
  - [Error Rates](#error-rates)
  - [Latency & Response Times](#latency--response-times)
- [Comparisons](#comparisons)
- [Time-Based Queries](#time-based-queries)
- [Aggregations](#aggregations)
- [Troubleshooting & Alerts](#troubleshooting--alerts)
- [Advanced Queries](#advanced-queries)

---

## Resource Metrics

### CPU Usage

#### Example 1: Basic CPU Query
**Natural Language:**
```
"What is the CPU usage for the auth service?"
```

**Generated PromQL:**
```promql
rate(container_cpu_usage_seconds_total{service="auth"}[5m])
```

**Sample Output:**
```json
{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [
      {
        "metric": {"service": "auth", "pod": "auth-7d9f4c5b-xk2p9"},
        "value": [1704121200, "0.45"]
      }
    ]
  }
}
```

**Why This Query:** CPU is a counter metric, so we use `rate()` to show usage per second over a 5-minute window.

---

#### Example 2: CPU Usage Across All Pods
**Natural Language:**
```
"Show me CPU usage for all pods in the production namespace"
```

**Generated PromQL:**
```promql
sum(rate(container_cpu_usage_seconds_total{namespace="production"}[5m])) by (pod)
```

**Use Case:** Identifying which pods are consuming the most CPU.

---

#### Example 3: High CPU Alert Query
**Natural Language:**
```
"Which services have CPU usage above 80%?"
```

**Generated PromQL:**
```promql
avg(rate(container_cpu_usage_seconds_total[5m])) by (service) > 0.8
```

**Use Case:** Quick identification of overloaded services during incidents.

---

### Memory Usage

#### Example 4: Basic Memory Query
**Natural Language:**
```
"How much memory is the payment service using?"
```

**Generated PromQL:**
```promql
sum(container_memory_usage_bytes{service="payment"}) / 1024 / 1024 / 1024
```

**Sample Output:**
```
4.7 GB
```

**Why This Query:** Memory is a gauge metric (doesn't need `rate()`), divided by 1024³ to convert to GB.

---

#### Example 5: Memory Usage Percentage
**Natural Language:**
```
"What percentage of memory is the database using?"
```

**Generated PromQL:**
```promql
100 * (
  container_memory_usage_bytes{service="database"} /
  container_spec_memory_limit_bytes{service="database"}
)
```

**Use Case:** Understanding if services are approaching memory limits.

---

### Disk I/O

#### Example 6: Disk Read Rate
**Natural Language:**
```
"Show me disk read rate for the database service"
```

**Generated PromQL:**
```promql
rate(container_fs_reads_bytes_total{service="database"}[5m]) / 1024 / 1024
```

**Sample Output:**
```
12.3 MB/s
```

---

#### Example 7: Disk Write Operations
**Natural Language:**
```
"How many disk write operations per second for storage nodes?"
```

**Generated PromQL:**
```promql
rate(container_fs_writes_total{job="storage"}[5m])
```

**Use Case:** Monitoring I/O patterns during heavy write operations.

---

### Network Traffic

#### Example 8: Network Ingress
**Natural Language:**
```
"What's the incoming network traffic for the API gateway?"
```

**Generated PromQL:**
```promql
rate(container_network_receive_bytes_total{service="api-gateway"}[5m]) / 1024 / 1024
```

**Sample Output:**
```
45.8 MB/s
```

---

#### Example 9: Network Egress by Namespace
**Natural Language:**
```
"Show me outgoing network traffic by namespace"
```

**Generated PromQL:**
```promql
sum(rate(container_network_transmit_bytes_total[5m])) by (namespace) / 1024 / 1024
```

**Use Case:** Cost analysis for cloud egress charges.

---

## Application Metrics

### Request Rates

#### Example 10: Basic Request Rate
**Natural Language:**
```
"How many requests per second is the API handling?"
```

**Generated PromQL:**
```promql
sum(rate(http_requests_total{service="api"}[5m]))
```

**Sample Output:**
```
1,247 requests/second
```

---

#### Example 11: Request Rate by Endpoint
**Natural Language:**
```
"Show me request rate broken down by endpoint"
```

**Generated PromQL:**
```promql
sum(rate(http_requests_total[5m])) by (endpoint)
```

**Use Case:** Identifying which endpoints receive the most traffic.

---

### Error Rates

#### Example 12: Basic Error Rate
**Natural Language:**
```
"What's the error rate for the payment service?"
```

**Generated PromQL:**
```promql
sum(rate(http_requests_total{service="payment", status=~"5.."}[5m])) /
sum(rate(http_requests_total{service="payment"}[5m])) * 100
```

**Sample Output:**
```
0.23% (23 errors per 10,000 requests)
```

**Why This Query:** Calculates percentage of 5xx responses vs total requests.

---

#### Example 13: Error Rate Above Threshold
**Natural Language:**
```
"Which services have error rates above 1%?"
```

**Generated PromQL:**
```promql
(
  sum(rate(http_requests_total{status=~"5.."}[5m])) by (service) /
  sum(rate(http_requests_total[5m])) by (service)
) > 0.01
```

**Use Case:** Quick health check across all services.

---

#### Example 14: 4xx vs 5xx Errors
**Natural Language:**
```
"Compare client errors vs server errors for the auth service"
```

**Generated PromQL:**
```promql
# 4xx (Client Errors)
sum(rate(http_requests_total{service="auth", status=~"4.."}[5m]))

# 5xx (Server Errors)
sum(rate(http_requests_total{service="auth", status=~"5.."}[5m]))
```

**Use Case:** Distinguishing between client issues and server issues.

---

### Latency & Response Times

#### Example 15: 95th Percentile Latency
**Natural Language:**
```
"What's the 95th percentile response time for the checkout service?"
```

**Generated PromQL:**
```promql
histogram_quantile(0.95,
  sum(rate(http_request_duration_seconds_bucket{service="checkout"}[5m])) by (le)
)
```

**Sample Output:**
```
0.342 seconds (342ms)
```

**Why This Query:** Uses histogram metric to calculate percentiles accurately.

---

#### Example 16: Average Latency Over Time
**Natural Language:**
```
"Show me average response time for the last hour"
```

**Generated PromQL:**
```promql
avg(rate(http_request_duration_seconds_sum[5m]) /
    rate(http_request_duration_seconds_count[5m]))
```

**Use Case:** Spotting latency trends.

---

#### Example 17: Latency by Endpoint
**Natural Language:**
```
"Which endpoints are slowest in the API service?"
```

**Generated PromQL:**
```promql
topk(5,
  histogram_quantile(0.95,
    sum(rate(http_request_duration_seconds_bucket{service="api"}[5m])) by (endpoint, le)
  )
)
```

**Use Case:** Optimization prioritization.

---

## Comparisons

#### Example 18: Service Comparison
**Natural Language:**
```
"Compare CPU usage between auth and payment services"
```

**Generated PromQL:**
```promql
sum(rate(container_cpu_usage_seconds_total{service=~"auth|payment"}[5m])) by (service)
```

**Sample Output:**
```
auth: 0.45 cores
payment: 0.82 cores
```

---

#### Example 19: Environment Comparison
**Natural Language:**
```
"Compare error rates in staging vs production"
```

**Generated PromQL:**
```promql
sum(rate(http_requests_total{status=~"5..", environment=~"staging|production"}[5m])) by (environment) /
sum(rate(http_requests_total{environment=~"staging|production"}[5m])) by (environment)
```

**Use Case:** Validating deployments.

---

#### Example 20: Before vs After Comparison
**Natural Language:**
```
"Compare latency now vs 1 hour ago"
```

**Generated PromQL:**
```promql
# Current
histogram_quantile(0.95,
  sum(rate(http_request_duration_seconds_bucket[5m])) by (le)
)

# 1 Hour Ago
histogram_quantile(0.95,
  sum(rate(http_request_duration_seconds_bucket[5m] offset 1h)) by (le)
)
```

**Use Case:** Impact analysis after deployments.

---

## Time-Based Queries

#### Example 21: Hourly Trends
**Natural Language:**
```
"Show me request rate for the last 24 hours"
```

**Generated PromQL:**
```promql
sum(rate(http_requests_total[5m]))
```

**Note:** Time range specified in query API call, not PromQL itself.

---

#### Example 22: Peak Usage
**Natural Language:**
```
"What was the peak CPU usage in the last week?"
```

**Generated PromQL:**
```promql
max_over_time(
  sum(rate(container_cpu_usage_seconds_total[5m]))[7d:5m]
)
```

**Use Case:** Capacity planning.

---

#### Example 23: Day-over-Day Comparison
**Natural Language:**
```
"Compare today's traffic to yesterday at the same time"
```

**Generated PromQL:**
```promql
# Today
sum(rate(http_requests_total[5m]))

# Yesterday same time
sum(rate(http_requests_total[5m] offset 24h))
```

**Use Case:** Detecting anomalies.

---

## Aggregations

#### Example 24: Total Across Namespace
**Natural Language:**
```
"What's the total memory usage for the production namespace?"
```

**Generated PromQL:**
```promql
sum(container_memory_usage_bytes{namespace="production"}) / 1024 / 1024 / 1024
```

**Sample Output:**
```
127.4 GB
```

---

#### Example 25: Average by Service
**Natural Language:**
```
"Show me average request rate per service"
```

**Generated PromQL:**
```promql
avg(rate(http_requests_total[5m])) by (service)
```

**Use Case:** Load distribution analysis.

---

#### Example 26: Top N Services
**Natural Language:**
```
"Which 5 services use the most memory?"
```

**Generated PromQL:**
```promql
topk(5, sum(container_memory_usage_bytes) by (service))
```

**Use Case:** Resource optimization targets.

---

## Troubleshooting & Alerts

#### Example 27: What's Breaking Right Now?
**Natural Language:**
```
"What's breaking right now?"
```

**Generated PromQL:**
```promql
# High error rates
(sum(rate(http_requests_total{status=~"5.."}[5m])) by (service) /
 sum(rate(http_requests_total[5m])) by (service)) > 0.05

# High CPU
avg(rate(container_cpu_usage_seconds_total[5m])) by (service) > 0.9

# High memory
(container_memory_usage_bytes / container_spec_memory_limit_bytes) > 0.9
```

**Use Case:** Incident triage.

---

#### Example 28: Service Health Check
**Natural Language:**
```
"Is the database service healthy?"
```

**Generated PromQL:**
```promql
# Multiple checks combined
up{service="database"} == 1 and
rate(http_requests_total{service="database", status=~"5.."}[5m]) /
rate(http_requests_total{service="database"}[5m]) < 0.01
```

**Sample Output:**
```
Healthy: Service is up and error rate < 1%
```

---

#### Example 29: Sudden Spike Detection
**Natural Language:**
```
"Show me services with sudden error rate spikes"
```

**Generated PromQL:**
```promql
(
  sum(rate(http_requests_total{status=~"5.."}[5m])) by (service) /
  sum(rate(http_requests_total{status=~"5.."}[5m] offset 10m)) by (service)
) > 2
```

**Use Case:** Anomaly detection (error rate doubled in last 5 minutes).

---

#### Example 30: Resource Exhaustion Warning
**Natural Language:**
```
"Which pods are close to running out of memory?"
```

**Generated PromQL:**
```promql
(container_memory_usage_bytes / container_spec_memory_limit_bytes) > 0.85
```

**Use Case:** Proactive scaling.

---

## Advanced Queries

#### Example 31: Multi-Metric Correlation
**Natural Language:**
```
"Show me services where high CPU correlates with high error rates"
```

**Generated PromQL:**
```promql
(avg(rate(container_cpu_usage_seconds_total[5m])) by (service) > 0.7) and
(sum(rate(http_requests_total{status=~"5.."}[5m])) by (service) /
 sum(rate(http_requests_total[5m])) by (service)) > 0.01
```

**Use Case:** Root cause analysis.

---

#### Example 32: Rate of Change
**Natural Language:**
```
"How fast is memory usage growing for the cache service?"
```

**Generated PromQL:**
```promql
deriv(container_memory_usage_bytes{service="cache"}[10m])
```

**Sample Output:**
```
+15.2 MB/minute
```

**Use Case:** Predicting resource needs.

---

#### Example 33: Percentile Comparison
**Natural Language:**
```
"Compare 50th, 95th, and 99th percentile latency"
```

**Generated PromQL:**
```promql
# P50
histogram_quantile(0.50, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))

# P95
histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))

# P99
histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))
```

**Use Case:** Understanding latency distribution.

---

## Using These Examples

### Try Them Out

```bash
# Example: CPU query
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query": "What is the CPU usage for the auth service?"}'

# Example: Error rate query
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query": "What'\''s the error rate for the payment service?"}'
```

### Customize for Your Metrics

Replace these placeholders with your actual values:
- **Services**: `auth`, `payment`, `api`, `database` → your service names
- **Namespaces**: `production`, `staging` → your namespaces
- **Metrics**: `http_requests_total` → your actual metric names
- **Labels**: `service`, `endpoint`, `status` → your label names

### Best Practices

1. **Be Specific**: "CPU for auth service" is better than "show me CPU"
2. **Include Time Context**: "in the last hour" vs just "CPU usage"
3. **Specify Thresholds**: "above 80%" gives clear filtering
4. **Use Comparisons**: "compare X vs Y" for differential analysis
5. **Ask Follow-ups**: If first query isn't perfect, refine it

---

## Common Query Patterns

### Resource Monitoring
- "What is [resource] for [service]?"
- "Show me [resource] across all [pods/services/namespaces]"
- "Which [entities] have [resource] above [threshold]?"

### Performance Analysis
- "What's the [metric] for [service]?"
- "Compare [metric] between [service A] and [service B]"
- "Show me [percentile] latency for [service]"

### Troubleshooting
- "What's breaking right now?"
- "Which services have errors above [threshold]?"
- "Show me [resource] trend over [time period]"

### Capacity Planning
- "What's the peak [resource] usage?"
- "How fast is [resource] growing?"
- "Which services use the most [resource]?"

---

## Need More Examples?

- **[Troubleshooting Guide](TROUBLESHOOTING.md)** - Examples for specific problems
- **[API Documentation](API.md)** - Programmatic query examples
- **[GitHub Discussions](https://github.com/seanankenbruck/observability-ai/discussions)** - Share your queries
- **Try asking!** - The system learns and improves

---

**Ready to query?** → [Quick Start](../QUICKSTART.md)
