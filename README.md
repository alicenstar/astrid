# Astrid

A fitness and nutrition tracking web app with dynamic calorie goals, workout split planning, and weekly adherence summaries.

Built with Go, HTMX, PostgreSQL, and Redis. Packaged as a Helm chart for Kubernetes deployment.

## Features

- **Authentication** -- Email/password signup, Google OAuth login, and a one-click demo login. Session-based auth stored in Redis with 7-day TTL.
- **Calorie Plans** -- Create and edit per-day calorie targets (e.g., higher on training days, lower on rest days). First plan auto-activates.
- **Daily Meal Log** -- Log meals with calories, protein, fiber, and cholesterol. Real-time progress bar and % daily value tracking.
- **Workout Splits** -- Define and edit weekly workout splits (e.g., Push/Pull/Legs) with planned exercises per day. First split auto-activates.
- **Workout Tracking** -- Mark today's workout complete from the dashboard. Track consecutive-day streaks.
- **Weekly Summary** -- 7-day calorie adherence table with color-coded percentages.
- **Dashboard** -- At-a-glance view of today's calorie progress, planned workout, streak, and a health data sync preview.
- **Dark/Light Mode** -- Toggle between themes; preference saved in localStorage.
- **Health Endpoint** -- `GET /healthz` returns structured JSON with Postgres and Redis connectivity status.

## Architecture

```
Go binary (chi router)
  ├── html/template + HTMX (server-rendered, no JS framework)
  ├── PostgreSQL (persistence: users, plans, logs, workouts)
  ├── Redis (sessions, caching: daily calorie rollups, workout streaks)
  └── Auth middleware (session cookie → user context)
```

Single binary serves the web UI, static assets, and API. Migrations run automatically on startup. Public routes (login, signup, healthz) bypass auth; all other routes require a valid session.

## Local Development

### Prerequisites

- **Go 1.23+** -- `go version`
- **PostgreSQL 16** -- running on `localhost:5432`
- **Redis 7** -- running on `localhost:6379`

The quickest way to get Postgres and Redis running locally is with Docker:

```bash
# Start Postgres
docker run -d --name astrid-pg \
  -e POSTGRES_USER=astrid \
  -e POSTGRES_PASSWORD=astrid \
  -e POSTGRES_DB=astrid \
  -p 5432:5432 \
  postgres:16-alpine

# Start Redis
docker run -d --name astrid-redis \
  -p 6379:6379 \
  redis:7-alpine

# Create test database (for running tests)
docker exec astrid-pg psql -U astrid -d postgres -c "CREATE DATABASE astrid_test OWNER astrid;"
```

### Run the app

```bash
cd /path/to/astrid
go run ./cmd/astrid/
```

The app starts on [http://localhost:8080](http://localhost:8080). Migrations run automatically. You'll see the login page -- use **Demo Login** for quick access.

For hot-reload during development:

```bash
go install github.com/air-verse/air@latest
air
```

### Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP listen port |
| `DATABASE_URL` | `postgres://astrid:astrid@localhost:5432/astrid?sslmode=disable` | PostgreSQL connection string |
| `REDIS_URL` | `redis://localhost:6379/0` | Redis connection string |
| `GOOGLE_CLIENT_ID` | _(empty)_ | Google OAuth client ID (optional) |
| `GOOGLE_CLIENT_SECRET` | _(empty)_ | Google OAuth client secret (optional) |
| `GOOGLE_REDIRECT_URL` | _(empty)_ | Google OAuth redirect URL (optional) |

When `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` are set, the Google login button appears on the login/signup pages. When unset, only email/password and demo login are available.

### Run tests

```bash
# Run all tests (use -p 1 to avoid cross-package DB conflicts)
go test ./... -p 1 -v

# Run specific package
go test ./internal/handlers/ -v
go test ./internal/models/ -v
go test ./internal/auth/ -v
```

Tests use the `astrid_test` database to avoid interfering with development data.

## Docker

Build the container image:

```bash
docker build -t astrid:dev .
```

The Dockerfile uses a multi-stage build: Go compilation in `golang:1.23-alpine`, then a minimal `alpine:3.20` runtime image with the binary, migrations, templates, and static files.

## Helm Chart

The Helm chart is in `chart/astrid/` and packages the app for Kubernetes with Bitnami subcharts for PostgreSQL and Redis.

### Lint

```bash
cd chart/astrid
helm dependency build
helm lint .
```

### Install (embedded databases -- default)

By default, the chart deploys PostgreSQL and Redis as subcharts:

```bash
helm install astrid ./chart/astrid \
  --set image.repository=<your-registry>/astrid \
  --set image.tag=dev
```

This starts three pods: the app, PostgreSQL, and Redis. An init container on the app pod waits for PostgreSQL to be reachable before the main container starts (prevents crash-loops on first install).

### Install (BYO databases)

To use your own external PostgreSQL and Redis instead of the embedded subcharts:

```bash
helm install astrid ./chart/astrid \
  --set image.repository=<your-registry>/astrid \
  --set image.tag=dev \
  --set postgresql.enabled=false \
  --set externalDatabase.host=my-pg.example.com \
  --set externalDatabase.port=5432 \
  --set externalDatabase.user=astrid \
  --set externalDatabase.password=secret \
  --set externalDatabase.database=astrid \
  --set redis.enabled=false \
  --set externalRedis.host=my-redis.example.com \
  --set externalRedis.port=6379
```

With `postgresql.enabled=false`, no PostgreSQL pod is deployed -- the app connects to the external instance. Same for Redis.

### Google OAuth (optional)

```bash
helm install astrid ./chart/astrid \
  --set auth.google.clientID=your-client-id \
  --set auth.google.clientSecret=your-secret \
  --set auth.google.redirectURL=https://astrid.example.com/auth/google/callback
```

### TLS / HTTPS

The chart supports three TLS modes via `tls.mode`:

| Mode | Description |
|------|-------------|
| `auto` | Uses cert-manager to automatically provision a certificate. Set `tls.certManager.issuerName` and `tls.certManager.issuerKind`. |
| `manual` | You provide a Kubernetes TLS secret. Set `tls.secretName` to the name of your secret containing `tls.crt` and `tls.key`. |
| `selfsigned` | Creates a cert-manager SelfSigned Issuer and Certificate. Useful for dev/staging. Requires cert-manager in the cluster. |
| `""` (empty) | No TLS at the ingress level (default). |

To enable HTTPS with cert-manager (auto):

```bash
helm install astrid ./chart/astrid \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=astrid.example.com \
  --set ingress.hosts[0].paths[0].path=/ \
  --set ingress.hosts[0].paths[0].pathType=Prefix \
  --set tls.mode=auto \
  --set tls.certManager.issuerName=letsencrypt-prod
```

To use a manually provided certificate:

```bash
# Create the TLS secret first
kubectl create secret tls astrid-tls \
  --cert=path/to/tls.crt \
  --key=path/to/tls.key

helm install astrid ./chart/astrid \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=astrid.example.com \
  --set ingress.hosts[0].paths[0].path=/ \
  --set ingress.hosts[0].paths[0].pathType=Prefix \
  --set tls.mode=manual \
  --set tls.secretName=astrid-tls
```

### Kubernetes Best Practices

The chart includes:

- **Liveness probe** -- `GET /healthz` every 15s (initial delay 10s)
- **Readiness probe** -- `GET /healthz` every 10s (initial delay 5s)
- **Resource requests and limits** on all containers (app, PostgreSQL, Redis, init container)
- **Init container** -- Waits for PostgreSQL to be reachable before starting the app
- **Data persistence** -- PostgreSQL uses a PVC; deleting the app pod does not lose data

### Values Schema

The chart includes `values.schema.json` for validation. Run `helm lint .` to verify values conform to the schema.

## Project Structure

```
astrid/
├── cmd/astrid/main.go           # Entry point: config, DB, router, auth middleware
├── internal/
│   ├── auth/                    # Session management, middleware, context helpers
│   ├── config/                  # Env-based configuration
│   ├── database/                # Postgres + Redis connections
│   ├── handlers/                # HTTP handlers (chi)
│   ├── models/                  # Data models + DB queries
│   └── templates/               # Go html/template files
├── migrations/                  # SQL migrations (golang-migrate)
├── static/css/                  # Stylesheet (dark/light mode)
├── chart/astrid/                # Helm chart
├── Dockerfile                   # Multi-stage build
└── go.mod
```
