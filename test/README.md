# Integration Tests

This directory contains integration tests for the Observability AI system.

## Overview

Integration tests verify end-to-end functionality including:
- Mimir service discovery
- Authentication flows (JWT, API keys, sessions)
- Rate limiting
- Database operations
- Complete workflow testing

## Running Integration Tests

### Run all integration tests:
```bash
go test -tags=integration ./test/...
```

### Run with verbose output:
```bash
go test -v -tags=integration ./test/...
```

### Run specific test:
```bash
go test -tags=integration ./test/... -run TestMimirDiscoveryIntegration
```

### Skip long-running tests:
```bash
go test -tags=integration -short ./test/...
```

## Integration vs Unit Tests

| Aspect | Unit Tests | Integration Tests |
|--------|-----------|------------------|
| Location | `internal/*/` | `test/` |
| Run with | `go test ./...` | `go test -tags=integration ./test/...` |
| Build tag | None | `//go:build integration` |
| Speed | Fast (milliseconds) | Moderate (seconds) |
| Dependencies | Mocked | Real or realistic mocks |
| Purpose | Component correctness | System integration |
| CI/CD | Every commit | PRs and nightly |

## Test Coverage

### 1. Mimir Discovery Integration
- **TestMimirDiscoveryIntegration**: Full discovery workflow
  - Mimir connection testing
  - Metric retrieval
  - Service discovery
  - Service-metric association
  - Namespace filtering

### 2. Authentication Integration
- **TestAuthenticatedAPIIntegration**: Complete auth flows
  - JWT authentication
  - API key authentication
  - Session authentication
  - Role-based access control
  - Expired credential handling

### 3. End-to-End Workflow
- **TestEndToEndDiscoveryFlow**: Complete discovery workflow
  - Setup mock Mimir
  - Connect and discover
  - Verify service creation
  - Validate metrics association

### 4. Rate Limiting
- **TestRateLimitingIntegration**: Rate limiting behavior
  - Request limiting
  - Window reset
  - Client isolation

## Test Structure

```
test/
├── README.md              # This file
└── integration_test.go    # All integration tests
```

## Mock Servers

Integration tests use mock HTTP servers to simulate external services:

- **MockMimirServer**: Simulates Mimir/Prometheus API
  - Returns test metrics
  - Returns test services
  - Returns test labels

- **MockSemanticMapper**: In-memory semantic mapper
  - No database required
  - Fast test execution
  - Isolated test data

## CI/CD Integration

### GitHub Actions Example:
```yaml
- name: Run Integration Tests
  run: go test -tags=integration ./test/...
```

### Skip in Development:
Integration tests are tagged and won't run with regular `go test ./...`

## Writing New Integration Tests

### Template:
```go
//go:build integration
// +build integration

func TestMyIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }

    // Setup
    // ...

    // Test
    // ...

    // Assert
    // ...
}
```

### Best Practices:
1. Use build tags: `//go:build integration`
2. Skip in short mode: `testing.Short()`
3. Clean up resources: `defer cleanup()`
4. Use realistic test data
5. Test complete workflows
6. Verify all outcomes

## Comparison with cmd/test-* Programs

### Old Approach (cmd/test-*)
- Manual programs
- Run with `make test-db`, etc.
- Not part of test suite
- No assertions
- Hard to automate

### New Approach (test/)
- Proper Go tests
- Run with `go test`
- Part of test suite
- Full assertions
- Easy to automate
- CI/CD friendly

## Migration from Manual Tests

The integration tests replace the manual test programs:

| Old | New | Status |
|-----|-----|--------|
| `cmd/test-db/main.go` | `test/integration_test.go` | ✅ Replaced |
| `cmd/test-integration/main.go` | `test/integration_test.go` | ✅ Replaced |
| `cmd/test-llm/main.go` | Unit tests for LLM client | ✅ Replaced |

## Benefits

1. **Automated**: Run in CI/CD pipelines
2. **Repeatable**: Same results every time
3. **Fast**: Use mocks, no external dependencies
4. **Reliable**: Full assertions and error checking
5. **Maintainable**: Standard Go testing patterns
6. **Documented**: Clear test names and structure

## Future Enhancements

Potential additions:
- [ ] Database integration tests (with test containers)
- [ ] Redis integration tests
- [ ] LLM client integration tests (with mock API)
- [ ] HTTP API integration tests
- [ ] Performance benchmarks
- [ ] Load testing
