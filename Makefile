.PHONY: all build run run-backend run-frontend frontend ui stop stop-backend stop-frontend test clean db-up db-down dev

BINARY_NAME=cumulus
HTTP_PORT ?= 8080
FRONTEND_PORT ?= 3000
DATABASE_URL ?= postgres://postgres:postgres@localhost:5432/cumulus?sslmode=disable
GOOGLE_SCOPES ?= openid,email,https://www.googleapis.com/auth/drive
ENCRYPTION_KEY ?= 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef

all: build

build:
	@echo "Building Cumulus backend binary..."
	go build -o $(BINARY_NAME) ./cmd/cumulus

# Alias for backend
run: run-backend

run-backend:
	@echo "Starting Cumulus backend API..."
	@echo "Backend API: http://localhost:$(HTTP_PORT)"
	HTTP_PORT=$(HTTP_PORT) \
	DATABASE_URL=$(DATABASE_URL) \
	GOOGLE_SCOPES=$(GOOGLE_SCOPES) \
	ENCRYPTION_KEY=$(ENCRYPTION_KEY) \
	go run ./cmd/cumulus

# Target to start frontend UI on port 3000
run-frontend:
	@echo "Starting Cumulus Frontend UI on http://localhost:$(FRONTEND_PORT)..."
	python3 -m http.server $(FRONTEND_PORT) --directory web

frontend: run-frontend
ui: run-frontend

dev: db-up run-backend

stop-backend:
	@echo "Stopping Cumulus backend (port $(HTTP_PORT))..."
	@pids="$$(lsof -t -i :$(HTTP_PORT) 2>/dev/null || true)"; \
	if [ -n "$$pids" ]; then \
		kill $$pids || true; \
	else \
		pkill -f "go run ./cmd/cumulus" || true; \
		pkill -f "/cumulus$$" || true; \
	fi

stop-frontend:
	@echo "Stopping Cumulus frontend (port $(FRONTEND_PORT))..."
	@pids="$$(lsof -t -i :$(FRONTEND_PORT) 2>/dev/null || true)"; \
	if [ -n "$$pids" ]; then \
		kill $$pids || true; \
	else \
		pkill -f "http.server $(FRONTEND_PORT)" || true; \
	fi

stop: stop-backend stop-frontend

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
