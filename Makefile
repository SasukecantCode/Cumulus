.PHONY: all build run test clean db-up db-down

BINARY_NAME=cumulus

all: build

build:
	@echo "Building Cumulus binary..."
	go build -o $(BINARY_NAME) ./cmd/cumulus

run:
	@echo "Starting Cumulus backend..."
	go run ./cmd/cumulus

db-up:
	@echo "Starting local PostgreSQL database container..."
	docker run -d --name cumulus-db -p 5432:5432 \
		-e POSTGRES_USER=postgres \
		-e POSTGRES_PASSWORD=postgres \
		-e POSTGRES_DB=cumulus \
		postgres:15-alpine || docker start cumulus-db

db-down:
	@echo "Stopping local PostgreSQL database container..."
	docker stop cumulus-db || true
	docker rm cumulus-db || true

test:
	@echo "Running tests..."
	go test ./...

clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
