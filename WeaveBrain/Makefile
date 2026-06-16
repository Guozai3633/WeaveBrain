# WeaveBrain - Development Commands

.PHONY: build run test migrate-up migrate-down docker-up docker-down clean

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
