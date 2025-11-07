# Observability AI Development Makefile

.PHONY: help setup test-db migrate clean build run-test-db dev start-backend start-frontend start-dev-docker run-query-processor build-web serve stop restart

help: ## Show this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-25s\033[0m %s\n", $$1, $$2}'

# Development Environment Commands
dev: setup migrate ## Start full development environment (backend + frontend)
	@echo "Starting full development environment..."
	@echo "Backend will start in background, frontend in foreground..."
	@$(MAKE) start-backend &
	@sleep 5  # Give backend time to start
	@$(MAKE) start-frontend

start-backend: ## Start backend services and query processor
	@echo "Starting backend services..."
	@echo "Backend API will be available at: http://localhost:8080"
	@set -a; source .env; set +a; go run cmd/query-processor/main.go

start-frontend: ## Start frontend development server
	@echo "Starting frontend development server..."
	@echo "Frontend will be available at: http://localhost:3000"
	cd web && npm install && npm run dev

start-dev-docker: ## Start everything with Docker Compose (including query processor)
	@echo "Starting development environment with Docker..."
	@cp deploy/configs/development.env .env
	@echo "Checking for Claude API key..."
	@set -a; source .env; set +a; \
	if [ -z "$$CLAUDE_API_KEY" ]; then \
		echo "❌ CLAUDE_API_KEY not found in development.env"; \
		echo "Please add CLAUDE_API_KEY=your-api-key to deploy/configs/development.env"; \
		exit 1; \
	fi
	docker-compose up -d
	@echo "Development environment started with Docker!"
	@echo "Frontend UI: http://localhost:3000"
	@echo "Backend API: http://localhost:8080"
	@echo "Health check: http://localhost:8080/health"
	@echo "Prometheus: http://localhost:9090"

run-query-processor: ## Run the query processor locally (requires setup first)
	@echo "Starting query processor..."
	@echo "Make sure you've run 'make setup migrate' first"
	@set -a; source .env; set +a; \
	if [ -z "$$CLAUDE_API_KEY" ]; then \
		echo "❌ CLAUDE_API_KEY not found in .env"; \
		echo "Please add it to deploy/configs/development.env and run 'make setup'"; \
		exit 1; \
	fi; \
	go run cmd/query-processor/main.go

# Original Commands (preserved)
setup: ## Start PostgreSQL and Redis for development
	@echo "Setting up environment..."
	@cp deploy/configs/development.env .env
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
	@echo "Waiting for Redis to be ready..."
	@until docker-compose -f docker-compose.test.yml exec -T redis redis-cli -a changeme ping > /dev/null 2>&1; do \
		echo "Redis is not ready yet, waiting..."; \
		sleep 2; \
	done
	@echo "Development environment is ready!"

migrate: ## Run database migrations
	@echo "Running database migrations..."
	@set -a; source .env; set +a; go run cmd/migrate/main.go
	@echo "Migrations completed!"

test-db: setup ## Run database tests with example data
	@echo "Running database tests..."
	@echo "Verifying database connectivity..."
	@docker-compose -f docker-compose.test.yml exec -T postgres pg_isready -U obs_ai -d observability_ai
	@echo "Database is ready, running tests..."
	@set -a; source .env; set +a; go run cmd/test-db/main.go

build: ## Build the query processor
	@echo "Building query processor..."
	@set -a; source .env; set +a; go build -o bin/query-processor cmd/query-processor/main.go

# Frontend Commands
build-web: ## Build the web interface for production
	@echo "Building web interface..."
	cd web && npm install && npm run build
	@echo "Web interface built in web/dist/"

serve: ## Serve the built web interface (for testing production build)
	@echo "Serving web interface on http://localhost:4173..."
	cd web && npm run preview

install-web: ## Install web dependencies
	@echo "Installing web dependencies..."
	cd web && npm install

# Utility Commands
stop: ## Stop all services
	@echo "Stopping all services..."
	docker-compose -f docker-compose.test.yml down
	docker-compose down 2>/dev/null || true
	@pkill -f "query-processor" || true
	@pkill -f "cmd/query-processor/main.go" || true
	@pkill -f "npm run dev" || true
	@echo "All services stopped"

restart: stop setup migrate ## Restart backend services
	@echo "Backend services restarted!"

clean: ## Stop and remove development environment
	@echo "Cleaning up development environment..."
	docker-compose -f docker-compose.test.yml down -v
	docker-compose down -v 2>/dev/null || true
	rm -rf bin/
	rm -rf web/dist/
	rm -rf web/node_modules/.cache/

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