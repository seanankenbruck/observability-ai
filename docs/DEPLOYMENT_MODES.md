# Deployment Modes

This document explains the different ways to run the Observability AI application.

## Quick Comparison

| Command | Backend | Frontend | Use Case |
|---------|---------|----------|----------|
| `make start-dev-docker` | Docker | Docker (nginx) | Quick demo, production-like environment |
| `make dev` | Local (Go) | Local (Vite dev server) | Active development with hot-reload |
| `make start-backend` | Local (Go) | None | Backend development only |
| `make start-frontend` | None | Local (Vite dev server) | Frontend development only |

## Detailed Breakdown

### 1. `make start-dev-docker` (Full Docker)

**What it does:**
- Starts **all services** in Docker containers
- Backend runs in a production-like container
- Frontend is built and served via nginx
- Includes PostgreSQL, Redis, Mimir

**When to use:**
- Quick demos or evaluation
- Don't have Go/Node installed locally
- Want to test production-like build
- Testing container orchestration

**Access:**
- Frontend UI: http://localhost:3000
- Backend API: http://localhost:8080

**Pros:**
- Complete isolated environment
- Closer to production setup
- No local dependencies needed
- Easy cleanup with `docker-compose down`

**Cons:**
- Slower to rebuild on code changes
- No hot-reload for development
- Larger disk space usage
- Longer initial startup time

---

### 2. `make dev` (Local Development)

**What it does:**
- Runs backend locally with Go
- Runs frontend locally with Vite dev server
- Uses Docker only for PostgreSQL, Redis, Mimir

**When to use:**
- Active development
- Need hot-reload for frontend
- Want fast iteration cycles
- Debugging Go code

**Access:**
- Frontend UI: http://localhost:3000 (Vite dev server with HMR)
- Backend API: http://localhost:8080 (Go process)

**Pros:**
- Fast hot-reload on file changes
- Better debugging experience
- Instant feedback loop
- Lower resource usage

**Cons:**
- Requires Go and Node.js installed
- More manual setup
- Multiple terminal windows

---

## Architecture Differences

### Docker Mode (`make start-dev-docker`)
```
┌─────────────────────────────────────────────────┐
│              Docker Network                     │
│                                                 │
│  ┌──────────┐  ┌──────────────┐  ┌──────────┐   │
│  │          │  │              │  │          │   │
│  │ Frontend │──│    Backend   │──│ Database │   │
│  │ (nginx)  │  │ (Go binary)  │  │          │   │
│  │          │  │              │  │          │   │
│  └────┬─────┘  └──────────────┘  └──────────┘   │
│       │                                         │
└───────┼─────────────────────────────────────────┘
        │
    Browser (:3000)
```

### Local Development Mode (`make dev`)
```
┌──────────────┐              ┌─────────────────────┐
│              │              │   Docker Network    │
│   Browser    │              │                     │
│              │              │  ┌──────────────┐   │
└──────┬───────┘              │  │              │   │
       │                      │  │   Database   │   │
       │ :3000                │  │              │   │
       ├──────────────┐       │  └──────────────┘   │
       │              │       │                    │
┌──────▼───────┐  ┌──▼─────┐ └─────────────────────┘
│              │  │        │           ▲
│   Vite Dev   │  │   Go   │───────────┘
│   Server     │  │Backend │
│ (Hot reload) │  │        │
└──────────────┘  └────────┘
   Host Process    Host Process
```

---

## Frontend Differences

### Docker Mode
- Frontend is **built** (`npm run build`)
- Served by **nginx** on port 3000
- Static files in production mode
- nginx proxies `/api` requests to backend
- No hot-reload

### Local Development Mode
- Frontend runs via **Vite dev server**
- Hot Module Replacement (HMR) enabled
- Instant updates on file save
- Vite proxies `/api` requests to backend
- Source maps for debugging

---

## Backend Differences

### Docker Mode
- Go binary runs inside container
- Environment variables from docker-compose.yml
- Connects to `postgres` hostname (Docker network)
- GIN_MODE=release (production mode)

### Local Development Mode
- Go runs directly on host
- Environment variables from `.env` file
- Connects to `localhost:5433` (Docker port mapping)
- GIN_MODE=debug (development mode)
- Better debugging with delve

---

## Migration Path

When you're ready to deploy to production:

1. Use `make start-dev-docker` to test the containerized setup
2. Both frontend and backend are already containerized
3. Update environment variables for production
4. Use a reverse proxy (nginx/Traefik) if needed
5. Add SSL certificates
6. Use production-grade PostgreSQL (not Docker)

---

## Common Issues

### Docker Mode
**Issue:** Frontend shows "Cannot connect to backend"
**Solution:** Ensure all containers are running: `docker-compose ps`

**Issue:** Changes not reflected
**Solution:** Rebuild containers: `docker-compose up -d --build`

### Local Development Mode
**Issue:** Port 3000 already in use
**Solution:** Find and kill the process: `lsof -ti:3000 | xargs kill`

**Issue:** Backend can't connect to database
**Solution:** Ensure Docker services are running: `make setup`
