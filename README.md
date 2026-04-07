# Astrid

A fitness and nutrition tracker built with Go, HTMX, PostgreSQL, and Redis. Packaged as a Helm chart for Kubernetes.

## Features

- **Calorie Plans** -- Per-day calorie targets (e.g., higher on training days, lower on rest). First plan auto-activates.
- **Daily Meal Log** -- Log meals with calories, protein, fiber, and cholesterol. Progress bar and % daily value tracking.
- **Workout Splits** -- Weekly workout splits with planned exercises per day. Mark workouts complete and track streaks.
- **Weekly Summary** -- 7-day calorie adherence table with color-coded percentages.
- **Dashboard** -- Today's calorie progress, planned workout, and streak at a glance.
- **Auth** -- Email/password, Google OAuth, and one-click demo login with sample data.
- **Health Endpoint** -- `GET /healthz` returns JSON with Postgres and Redis status.

## Quick Start

Requires Docker (or OrbStack) and Go 1.26+.

```bash
make dev    # Start Postgres + Redis containers, run app with hot-reload
```

Opens at [http://localhost:8080](http://localhost:8080). Click **Demo Login** for instant access with sample data.

```bash
make test   # Run all tests
make push   # Build + push Docker image (linux/amd64)
make lint   # Helm lint
make clean  # Stop containers, remove artifacts
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP listen port |
| `DATABASE_URL` | `postgres://astrid:astrid@localhost:5432/astrid?sslmode=disable` | PostgreSQL connection string |
| `REDIS_URL` | `redis://localhost:6379/0` | Redis connection string |
| `GOOGLE_CLIENT_ID` | _(empty)_ | Google OAuth client ID (optional -- enables Google login button) |
| `GOOGLE_CLIENT_SECRET` | _(empty)_ | Google OAuth client secret (optional) |
| `GOOGLE_REDIRECT_URL` | _(empty)_ | Google OAuth redirect URL (optional) |

## Helm Chart

The chart is in `chart/astrid/` with Bitnami subcharts for PostgreSQL and Redis.

### Install (embedded databases -- default)

```bash
helm install astrid ./chart/astrid \
  --set image.repository=<your-registry>/astrid \
  --set image.tag=latest
```

Deploys three pods: the app, PostgreSQL, and Redis. An init container waits for PostgreSQL before the app starts.

### Install (BYO databases)

Disable the embedded subcharts and point to your own instances:

```bash
helm install astrid ./chart/astrid \
  --set postgresql.enabled=false \
  --set externalDatabase.host=my-pg.example.com \
  --set externalDatabase.user=astrid \
  --set externalDatabase.password=secret \
  --set redis.enabled=false \
  --set externalRedis.host=my-redis.example.com
```

### TLS

Set `tls.mode` to control HTTPS behavior (requires `ingress.enabled=true`):

| Mode | Description |
|------|-------------|
| `auto` | cert-manager provisions a certificate automatically |
| `manual` | You provide a TLS secret via `tls.secretName` |
| `selfsigned` | cert-manager generates a self-signed cert (good for dev/staging) |
| _(empty)_ | No TLS (default) |

### Other Chart Features

- Liveness and readiness probes on `/healthz`
- Resource requests and limits on all containers
- `values.schema.json` for input validation (`helm lint` checks it)
- Configurable service type, ingress, and Google OAuth via values

## Project Structure

```
astrid/
├── cmd/astrid/main.go           # Entry point
├── internal/
│   ├── auth/                    # Session management + middleware
│   ├── config/                  # Env-based configuration
│   ├── database/                # Postgres + Redis connections
│   ├── handlers/                # HTTP handlers
│   ├── models/                  # Data models + queries
│   └── templates/               # HTML templates
├── migrations/                  # SQL migrations (golang-migrate)
├── static/css/                  # Stylesheet
├── chart/astrid/                # Helm chart
├── Dockerfile                   # Multi-stage build
└── go.mod
```
