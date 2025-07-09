# Observability AI Development Makefile

.PHONY: help setup test-db migrate clean build run-test-db

# Default environment variables
export DB_HOST ?= localhost
export DB_PORT ?= 5433
export DB_NAME ?= observability_ai
export DB_USER ?= obs_ai
export DB_PASSWORD ?= changeme
export DB_SSLMODE ?= disable

help: ## Show this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

setup: ## Start PostgreSQL and Redis for development
	@echo "Starting development environment..."
	docker-compose -f docker-compose.test.yml up -d
	@echo "Waiting for PostgreSQL to be ready..."
	@until docker-compose -f docker-compose.test.yml exec -T postgres pg_isready -U obs_ai -d observability_ai; do \
		echo "PostgreSQL is not ready yet, waiting..."; \
		sleep 2; \
	done
	@echo "Verifying database user and permissions..."
	@until docker-compose -f docker-compose.test.yml exec -T postgres psql -U obs_ai -d observability_ai -c "SELECT 1;" > /dev/null 2>&1; do \
		echo "Database user not ready yet, waiting..."; \
		sleep 2; \
	done
	@echo "Development environment is ready!"

migrate: ## Run database migrations
	@echo "Running database migrations..."
	go run cmd/test-db/main.go || true
	@echo "Migrations completed!"

test-db: setup ## Run database tests with example data
	@echo "Running database tests..."
	@echo "Verifying database connectivity..."
	@docker-compose -f docker-compose.test.yml exec -T postgres pg_isready -U obs_ai -d observability_ai
	@echo "Database is ready, running tests..."
	go run cmd/test-db/main.go

build: ## Build the query processor
	@echo "Building query processor..."
	go build -o bin/query-processor cmd/query-processor/main.go

clean: ## Stop and remove development environment
	@echo "Cleaning up development environment..."
	docker-compose -f docker-compose.test.yml down -v
	rm -rf bin/

logs: ## Show logs from development environment
	docker-compose -f docker-compose.test.yml logs -f

psql: ## Connect to PostgreSQL with psql
	docker-compose -f docker-compose.test.yml exec postgres psql -U obs_ai -d observability_ai

redis-cli: ## Connect to Redis with redis-cli
	docker-compose -f docker-compose.test.yml exec redis redis-cli -a changeme

deps: ## Download Go dependencies
	go mod download
	go mod tidy

fmt: ## Format Go code
	go fmt ./...

lint: ## Run linter
	go vet ./...

test-unit: ## Run unit tests
	go test -v ./...

# Database inspection commands
health-check: ## Check if PostgreSQL is ready
	@echo "Checking PostgreSQL health..."
	@docker-compose -f docker-compose.test.yml exec -T postgres pg_isready -U obs_ai -d observability_ai || (echo "PostgreSQL is not ready" && exit 1)
	@echo "Testing database user permissions..."
	@docker-compose -f docker-compose.test.yml exec -T postgres psql -U obs_ai -d observability_ai -c "SELECT 1;" > /dev/null 2>&1 || (echo "Database user permissions not ready" && exit 1)
	@echo "PostgreSQL is healthy!"

db-services: ## List all services in database
	docker-compose -f docker-compose.test.yml exec postgres psql -U obs_ai -d observability_ai -c "SELECT name, namespace, labels->>'team' as team FROM services ORDER BY name;"

db-metrics: ## List all metrics in database
	docker-compose -f docker-compose.test.yml exec postgres psql -U obs_ai -d observability_ai -c "SELECT m.name, m.type, s.name as service FROM metrics m JOIN services s ON m.service_id = s.id ORDER BY s.name, m.name;"

db-embeddings: ## List query embeddings
	docker-compose -f docker-compose.test.yml exec postgres psql -U obs_ai -d observability_ai -c "SELECT query_text, LEFT(promql_template, 50) as promql FROM query_embeddings ORDER BY created_at;"

# Quick start command
start: setup migrate test-db ## Start everything and run tests