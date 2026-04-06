.PHONY: dev stop start db test lint build clean docker push

# Start Postgres + Redis, run the app with hot-reload
dev: start
	~/go/bin/air

# Start Postgres + Redis containers (idempotent)
start:
	@docker start astrid-pg 2>/dev/null || docker run -d --name astrid-pg \
		-e POSTGRES_USER=astrid -e POSTGRES_PASSWORD=astrid -e POSTGRES_DB=astrid \
		-p 5432:5432 postgres:16-alpine
	@docker start astrid-redis 2>/dev/null || docker run -d --name astrid-redis \
		-p 6379:6379 redis:7-alpine
	@sleep 1
	@docker exec astrid-pg psql -U astrid -d postgres -tc \
		"SELECT 1 FROM pg_database WHERE datname = 'astrid_test'" | grep -q 1 || \
		docker exec astrid-pg psql -U astrid -d postgres -c "CREATE DATABASE astrid_test OWNER astrid;"
	@echo "Postgres and Redis running"

# Stop containers
stop:
	@docker stop astrid-pg astrid-redis 2>/dev/null || true
	@echo "Stopped"

# Run all tests
test: start
	go test ./... -p 1 -count=1

# Helm lint
lint:
	cd chart/astrid && helm dependency build 2>/dev/null; helm lint .

# Build binary
build:
	go build -o astrid ./cmd/astrid/

# Build Docker image (AMD64 for Kubernetes)
docker:
	docker build --platform linux/amd64 -t alicenstar/astrid:latest .

# Push Docker image
push: docker
	docker push alicenstar/astrid:latest

# Remove containers and built artifacts
clean: stop
	@docker rm astrid-pg astrid-redis 2>/dev/null || true
	@rm -f astrid
	@rm -rf tmp/
	@echo "Cleaned"
