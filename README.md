# GitPlane

Managed Kubernetes platform as a service. Connect your git repo, pick services from a curated catalog, and let FluxCD handle the rest.

## Architecture

- **Web UI** (Next.js + Tailwind) — user-facing dashboard
- **API Server** (Go + Echo) — REST API with JWT auth
- **Agent** (Go) — lightweight binary deployed in customer clusters, reads FluxReport CR and reports status
- **GitOps** — all manifests committed to git, FluxCD reconciles

## Quick Start

```bash
# Start all services locally
cd docker
docker compose up -d

# API runs on :8080, web on :3000, Postgres on :5432
```

## Project Structure

```
├── api/              # HTTP API server (Echo)
│   ├── handlers/     # Request handlers (auth, clusters, services, git, agent)
│   ├── middleware/    # JWT auth, RBAC, agent auth
│   └── ws/           # WebSocket for real-time status
├── agent/            # Cluster agent binary
├── cmd/server/       # API server entrypoint
├── deploy/           # Helm chart
├── docker/           # Dockerfiles + docker-compose
├── gitops/           # Git integration (GitHub/GitLab API clients)
├── migrations/       # PostgreSQL migrations
├── pkg/              # Shared libraries
│   ├── catalog/      # Curated service catalog
│   ├── config/       # Platform config model
│   ├── generator/    # Flux manifest generator
│   └── schema/       # JSON schema generation
└── web/              # Next.js frontend
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/auth/register` | Create org + admin user |
| POST | `/api/v1/auth/login` | Login |
| POST | `/api/v1/auth/refresh` | Refresh JWT |
| GET | `/api/v1/clusters` | List clusters |
| POST | `/api/v1/clusters` | Create cluster |
| GET | `/api/v1/clusters/:id` | Cluster detail + status |
| PUT | `/api/v1/clusters/:id` | Update cluster |
| DELETE | `/api/v1/clusters/:id` | Delete cluster |
| GET | `/api/v1/clusters/:id/agent-install` | Agent install YAML |
| GET | `/api/v1/clusters/:id/status` | Live FluxReport data |
| GET | `/api/v1/clusters/:id/history` | Generation history |
| GET | `/api/v1/catalog` | Service catalog |
| GET | `/api/v1/clusters/:id/services` | Cluster services |
| PUT | `/api/v1/clusters/:id/services/:name` | Toggle service |
| POST | `/api/v1/clusters/:id/generate` | Generate & commit manifests |
| POST | `/api/v1/clusters/:id/generate/preview` | Preview manifests |
| POST | `/api/v1/git/connect` | Connect git repo |
| GET | `/api/v1/git/status` | Connection status |
| POST | `/api/v1/agent/report` | Agent status report |

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | `postgres://localhost:5432/gitplane?sslmode=disable` |
| `GITPLANE_JWT_SECRET` | JWT signing secret | required |
| `PORT` | API server port | `8080` |
| `GITPLANE_API_URL` | Agent: API URL to report to | required |
| `GITPLANE_AGENT_TOKEN` | Agent: authentication token | required |
| `GITPLANE_REPORT_INTERVAL` | Agent: reporting interval | `60s` |
