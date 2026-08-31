# WeaveBrain - Development Commands

.PHONY: build run test migrate-up migrate-down docker-up docker-down clean db-create-test migrate-test-up test-integration test-integration-fast

# Test database connection string (dedicated DB, not the dev DB).
TEST_DB_URL ?= postgresql://weavebrain:weavebrain_dev@localhost:5432/weavebrain_test

# Build the application
build:
	go build -o bin/weavebrain ./cmd/weavebrain

# Run the application
run: build
	./bin/weavebrain

# Run tests
test:
	go test ./... -v

# Run tests with coverage
test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out

# Database migrations
migrate-up:
	goose -dir internal/db/migration postgres "postgresql://weavebrain:weavebrain_dev@localhost:5432/weavebrain" up

migrate-down:
	goose -dir internal/db/migration postgres "postgresql://weavebrain:weavebrain_dev@localhost:5432/weavebrain" down

migrate-status:
	goose -dir internal/db/migration postgres "postgresql://weavebrain:weavebrain_dev@localhost:5432/weavebrain" status

# Integration test database (dedicated weavebrain_test DB, created idempotently
# inside the running postgres container; never touches the dev weavebrain DB).
db-create-test:
	docker exec weavebrain-postgres psql -U weavebrain -d weavebrain -tAc "SELECT 1 FROM pg_database WHERE datname='weavebrain_test'" | grep -q 1 || docker exec weavebrain-postgres createdb -U weavebrain weavebrain_test

# Apply goose migrations to the test database.
migrate-test-up:
	goose -dir internal/db/migration postgres "$(TEST_DB_URL)" up

# Run the full unit + integration suite against a real PostgreSQL test DB.
# Integration tests gate on WEAVEBRAIN_TEST_DATABASE_URL and skip without it.
test-integration: db-create-test migrate-test-up
	WEAVEBRAIN_TEST_DATABASE_URL="$(TEST_DB_URL)" go test ./... -count=1 -v

# Run only the real-PostgreSQL integration tests (skips plain unit tests).
test-integration-fast: db-create-test migrate-test-up
	WEAVEBRAIN_TEST_DATABASE_URL="$(TEST_DB_URL)" go test ./... -count=1 -run 'Integration|Soak|Orphan|Traceability|Acceptance|RealPG' -v

# Docker commands
docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out

# Format code
fmt:
	go fmt ./...

# Lint code (requires golangci-lint)
lint:
	golangci-lint run

# Tidy dependencies
tidy:
	go mod tidy
