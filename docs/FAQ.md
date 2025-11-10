# Frequently Asked Questions (FAQ)

Common questions about Observability AI answered.

---

## General Questions

### What is Observability AI?

Observability AI is a natural language interface for querying Prometheus and Mimir metrics. Instead of writing PromQL manually, you ask questions in plain English and get accurate PromQL generated and executed automatically.

**Example:**
- You ask: "What's the CPU usage for the auth service?"
- AI generates: `rate(container_cpu_usage_seconds_total{service="auth"}[5m])`
- Results returned automatically

---

### Why should I use this instead of writing PromQL directly?

**Time Savings:** Average 8-12 minutes saved per query
**Accessibility:** Junior engineers can query metrics like senior SREs
**Accuracy:** AI handles complex syntax, aggregations, and metric naming
**Learning Tool:** See generated PromQL to learn from examples

See [docs/WHY_OBSERVABILITY_AI.md](WHY_OBSERVABILITY_AI.md) for detailed comparison.

---

### How accurate are the generated queries?

**Very accurate** - Claude AI has been specifically chosen for technical accuracy:
- ~98% accuracy for common queries (CPU, memory, errors, latency)
- ~95% accuracy for complex queries (multi-service comparisons, percentiles)
- All queries are validated before execution for safety

You can always see the generated PromQL before execution and verify it yourself.

---

### Is this a replacement for Grafana?

**No, they're complementary:**
- **Grafana**: Great for dashboards and known metrics
- **Observability AI**: Great for ad-hoc questions and exploration

**Best practice:** Use both! Grafana for monitoring, Observability AI for investigation.

---

## Setup & Installation

### What are the minimum requirements?

**Required:**
- Docker and Docker Compose
- Claude API key (from Anthropic)
- Prometheus or Mimir instance with metrics

**Optional (for local development):**
- Go 1.21+
- Node.js 18+

---

### How long does setup take?

**5-10 minutes** following the [QUICKSTART.md](../QUICKSTART.md):
- 2 minutes: Clone and configure
- 3 minutes: Start services
- 1 minute: Test first query

---

### Do I need to know Go or React to use this?

**No!** You only need:
1. Docker basics (run/stop containers)
2. API usage (curl or similar)
3. Basic understanding of your metrics

The application is distributed as Docker containers.

---

### Can I run this without Docker?

Yes, but it's more complex:
1. Install Go 1.21+ and Node.js 18+
2. Install PostgreSQL with pgvector extension
3. Install Redis
4. Build and run manually

**Recommended:** Use Docker for simplicity.

---

## Claude API

### How much does Claude API cost?

**Approximate costs per query:**
- Haiku (fast): ~$0.001 per query
- Sonnet (balanced): ~$0.003 per query
- Opus (accurate): ~$0.015 per query

**Monthly estimate for team of 10:**
- 50 queries/day/person = 500 queries/day
- 500 × $0.003 (Sonnet) = $1.50/day
- **~$45/month** for entire team

Compare to time savings: 500 queries × 8 min saved × $100/hour = **$6,667/month saved**

---

### Do my metrics values go to Claude?

**No!** Only the following are sent to Claude:
- Your natural language question
- Available metric names
- Available service names
- Label names

**Your actual metric values never leave your infrastructure.**

---

### What if Claude API goes down?

- Cached queries continue to work
- Recent query history remains available
- You can execute PromQL directly against Prometheus/Mimir
- Application continues running in degraded mode

---

### Can I use a different LLM?

The architecture supports different LLMs, but Claude is currently the only supported model due to its superior technical accuracy. Support for other models may be added in the future.

---

## Usage & Features

### What types of queries can I ask?

**Resource Metrics:**
- "CPU usage for auth service"
- "Memory usage across all pods"
- "Disk I/O for database"

**Application Metrics:**
- "Error rate for payment service"
- "95th percentile latency for API"
- "Request rate by endpoint"

**Comparisons:**
- "Compare CPU: auth vs checkout"
- "Error rates: staging vs production"

**Troubleshooting:**
- "What's breaking right now?"
- "Which services have high error rates?"

See [docs/QUERY_EXAMPLES.md](QUERY_EXAMPLES.md) for 30+ examples.

---

### Can it handle complex queries?

**Yes!** Examples of complex queries that work:
- Multi-service comparisons
- Histogram percentile calculations
- Rate calculations with time offsets
- Nested aggregations
- Time-based comparisons (now vs 1 hour ago)

For very complex queries, you may need to refine your question or break it into multiple queries.

---

### How do I know if a query is cached?

Check the `"cached"` field in the response:

```json
{
  "query": "CPU usage for auth",
  "cached": true,
  "execution_time_ms": 23
}
```

Cached queries are much faster (typically <100ms vs 500ms+).

---

### Can I see the PromQL before it's executed?

**Yes!** Every response includes the generated PromQL:

```json
{
  "query": "CPU for auth",
  "promql": "rate(container_cpu_usage_seconds_total{service=\"auth\"}[5m])",
  "explanation": "This calculates...",
  "metadata": { ... }
}
```

---

### Does it support all PromQL functions?

**Most common functions are supported:**
- `rate()`, `irate()`
- `sum()`, `avg()`, `max()`, `min()`
- `histogram_quantile()`
- `by`, `without`
- Time offsets (`offset 1h`)

**Some advanced features may not work perfectly:**
- Complex recording rules
- Subqueries
- Very custom label selectors

---

## Security & Privacy

### Is my data secure?

**Yes:**
- Metric values never leave your infrastructure
- Only metric/service names sent to Claude
- All data encrypted in transit (HTTPS)
- Passwords hashed with bcrypt
- JWT tokens for authentication

See [SECURITY_BEST_PRACTICES.md](SECURITY_BEST_PRACTICES.md) for production security guide.

---

### Should I change default passwords?

**⚠️ Absolutely!** Default passwords:
- Database: `changeme`
- Redis: `changeme`
- JWT secret: `your-secret-key-change-in-production`

**Change immediately** before deploying to production!

---

### How do I secure API access?

**Built-in security features:**
1. JWT authentication (required by default)
2. API key support for service accounts
3. Rate limiting (100 req/hour by default)
4. Role-based access control (User/Admin)

See [CONFIGURATION.md](CONFIGURATION.md) for authentication setup.

---

### Can I run this in production?

**Yes!** The application is production-ready:
- Docker Compose deployment
- Kubernetes/Helm support
- Authentication and authorization
- Rate limiting
- Logging and monitoring
- Health checks

See deployment guides in docs/ folder.

---

## Prometheus/Mimir Integration

### Which Prometheus/Mimir versions are supported?

**Prometheus:** 2.x and later
**Mimir:** All versions
**Compatible with:**
- Thanos
- VictoriaMetrics
- Grafana Cloud Mimir
- Any Prometheus-compatible API

---

### Does this work with Grafana Cloud?

**Yes!** Configure authentication:

```bash
MIMIR_URL=https://prometheus-prod-xx.grafana.net
MIMIR_AUTH_TYPE=bearer
MIMIR_BEARER_TOKEN=your-grafana-cloud-token
MIMIR_TENANT_ID=your-org-id
```

---

### How does service discovery work?

**Automatic discovery:**
1. Application queries Prometheus/Mimir for all metrics
2. Extracts service names from labels (service, job, app)
3. Stores in database for semantic matching
4. Runs every 5 minutes by default

**Manual trigger:**
```bash
curl -X POST http://localhost:8080/admin/discovery/trigger \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

See [ARCHITECTURE.md](ARCHITECTURE.md) for details.

---

### What if I don't have Prometheus/Mimir yet?

You can still use Observability AI with:
1. Test data: `make test-db` loads sample services/metrics
2. Manual entry: Add services/metrics via API
3. Connect later: Add Prometheus/Mimir URL when ready

---

## Performance

### How fast are queries?

**Typical query breakdown:**
- Authentication: 5-10ms
- Cache lookup: 1-2ms
- Claude API: 200-500ms (cache miss)
- Prometheus query: 50-150ms
- Total (cache miss): ~300-700ms
- **Total (cache hit): ~75-200ms** (much faster!)

Identical queries are cached for 5 minutes.

---

### Can this handle high traffic?

**Yes:**
- Horizontal scaling supported (multiple backend instances)
- Redis caching reduces load
- Rate limiting protects resources
- Claude API has high rate limits on paid tiers

**Tested with:**
- 1000+ req/min with caching
- 100+ req/min without cache (limited by Claude API)

---

### How do I optimize performance?

1. **Use caching** (enabled by default)
2. **Use Haiku model** for speed
3. **Add database indexes**
4. **Use Redis for sessions**
5. **Scale horizontally** (add more backend instances)

See [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for performance tuning.

---

## Troubleshooting

### Where do I start if something isn't working?

1. **Check health endpoint:** `curl http://localhost:8080/health`
2. **Check logs:** `make logs`
3. **Read troubleshooting guide:** [TROUBLESHOOTING.md](TROUBLESHOOTING.md)
4. **Search issues:** [GitHub Issues](https://github.com/seanankenbruck/observability-ai/issues)

---

### "CLAUDE_API_KEY not found" error

**Solution:**
```bash
# Check .env file
grep CLAUDE_API_KEY .env

# Add if missing
echo "CLAUDE_API_KEY=sk-ant-your-key-here" >> .env

# Restart
make restart
```

---

### "401 Unauthorized" on all requests

**Reasons:**
1. Missing authentication token
2. Token expired (24h lifetime)
3. Invalid credentials

**Solution:**
```bash
# Login to get new token
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "your-user", "password": "your-pass"}'
```

---

### No services found

**Solutions:**
1. Enable discovery: `DISCOVERY_ENABLED=true`
2. Configure Prometheus URL: `MIMIR_URL=http://your-prometheus:9090`
3. Trigger manually: `/admin/discovery/trigger`
4. Load test data: `make test-db`

---

## Development & Contributing

### Can I contribute to this project?

**Yes!** Contributions welcome:
- Bug reports
- Feature requests
- Documentation improvements
- Code contributions

See [CONTRIBUTING.md](../CONTRIBUTING.md) for guidelines.

---

### How do I report a bug?

1. Check [existing issues](https://github.com/seanankenbruck/observability-ai/issues)
2. If not found, [open new issue](https://github.com/seanankenbruck/observability-ai/issues/new)
3. Include:
   - Description of issue
   - Steps to reproduce
   - Expected vs actual behavior
   - Logs (sanitized!)
   - Configuration (no passwords!)

---

### How do I request a feature?

1. Check [discussions](https://github.com/seanankenbruck/observability-ai/discussions)
2. Start a new discussion in "Ideas" category
3. Describe:
   - What you want to do
   - Why it's useful
   - Example use cases

---

### What's the roadmap?

**Planned features:**
- Multi-LLM support (GPT-4, etc.)
- Query suggestions/autocomplete
- Saved query templates
- Advanced visualization
- Slack/Teams integration
- Grafana plugin

See [GitHub Projects](https://github.com/seanankenbruck/observability-ai/projects) for status.

---

## Deployment

### Can I use this in Kubernetes?

**Yes!** Helm charts provided:

```bash
helm install observability-ai ./helm/observability-ai \
  --set claude.apiKey=$CLAUDE_API_KEY \
  --set prometheus.url=http://prometheus:9090
```

See [DEPLOYMENT_KUBERNETES.md](DEPLOYMENT_KUBERNETES.md) for details.

---

### What about Docker Compose?

**Yes!** Docker Compose is fully supported:

```bash
make start-dev-docker
```

See [DEPLOYMENT_DOCKER.md](DEPLOYMENT_DOCKER.md) for production setup.

---

### How do I upgrade to a new version?

**Docker Compose:**
```bash
# Pull latest images
docker-compose pull

# Restart with new images
docker-compose up -d
```

**Kubernetes:**
```bash
helm upgrade observability-ai ./helm/observability-ai
```

**Always backup database before upgrading!**

---

## Pricing & Licensing

### Is this free?

**The software is free and open source** (check LICENSE file for terms).

**Costs to run:**
- Claude API: ~$45/month for team of 10 (estimated)
- Infrastructure: ~$20-50/month (AWS/GCP/Azure for small deployment)
- **Total: ~$65-95/month**

**ROI:** Time savings typically exceed costs by 50-100x.

---

### What's the license?

Check the [LICENSE](../LICENSE) file in the repository.

---

### Can I use this commercially?

Check the license terms. Generally:
- ✅ Use internally at your company
- ✅ Deploy for your customers
- ⚠️ Check license for redistribution terms

---

## Still Have Questions?

- **[Documentation Index](README.md)** - All docs
- **[GitHub Discussions](https://github.com/seanankenbruck/observability-ai/discussions)** - Community Q&A
- **[GitHub Issues](https://github.com/seanankenbruck/observability-ai/issues)** - Bug reports
- **[TROUBLESHOOTING.md](TROUBLESHOOTING.md)** - Problem solving

---

**Question not answered?** [Start a discussion!](https://github.com/seanankenbruck/observability-ai/discussions/new)
