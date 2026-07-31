.PHONY: all build run stop test clean db-up db-down

BINARY_NAME=cumulus
HTTP_PORT ?= 8080
DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/cumulus?sslmode=disable
GOOGLE_SCOPES ?= openid,email,https://www.googleapis.com/auth/drive
ENCRYPTION_KEY ?= 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef

all: build

build:
	@echo "Building Cumulus binary..."
	go build -o $(BINARY_NAME) ./cmd/cumulus

run:
	@echo "Starting Cumulus backend..."
	@echo "Local host: http://localhost:$(HTTP_PORT)"
	HTTP_PORT=$(HTTP_PORT) \
	DATABASE_URL=$(DATABASE_URL) \
	GOOGLE_SCOPES=$(GOOGLE_SCOPES) \
	ENCRYPTION_KEY=$(ENCRYPTION_KEY) \
	go run ./cmd/cumulus

dev: db-up run

stop:
	@echo "Stopping Cumulus backend and freeing port $(HTTP_PORT)..."
	@pids="$$(lsof -t -i :$(HTTP_PORT) 2>/dev/null || true)"; \
	if [ -n "$$pids" ]; then \
		kill $$pids || true; \
	else \
		pkill -f "go run ./cmd/cumulus" || true; \
		pkill -f "/cumulus$$" || true; \
	fi

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
