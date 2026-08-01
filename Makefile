.PHONY: all build run run-backend run-frontend frontend ui stop stop-backend stop-frontend test clean db-up db-stop db-down dev

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

# Ensure Docker PostgreSQL database container is started
db-up:
	@echo "Ensuring PostgreSQL database container (cumulus-db) is running..."
	@if [ $$(docker ps -q -f name=^cumulus-db$$ | wc -l) -eq 0 ]; then \
		if [ $$(docker ps -a -q -f name=^cumulus-db$$ | wc -l) -gt 0 ]; then \
			docker start cumulus-db >/dev/null; \
		else \
			docker run -d --name cumulus-db -p 5432:5432 \
				-e POSTGRES_USER=postgres \
				-e POSTGRES_PASSWORD=postgres \
				-e POSTGRES_DB=cumulus \
				postgres:15-alpine >/dev/null; \
		fi \
	fi
	@echo "PostgreSQL database container is running."

# Stop Docker DB container
db-stop:
	@echo "Stopping PostgreSQL database container..."
	@docker stop cumulus-db >/dev/null 2>&1 || true

# Remove Docker DB container
db-down: db-stop
	@echo "Removing PostgreSQL database container..."
	@docker rm cumulus-db >/dev/null 2>&1 || true

# Target to run standalone backend
run-backend: db-up
	@echo "Starting Cumulus backend API on http://localhost:$(HTTP_PORT)..."
	HTTP_PORT=$(HTTP_PORT) \
	DATABASE_URL=$(DATABASE_URL) \
	GOOGLE_SCOPES=$(GOOGLE_SCOPES) \
	ENCRYPTION_KEY=$(ENCRYPTION_KEY) \
	go run ./cmd/cumulus

# Target to run standalone frontend
run-frontend:
	@echo "Starting Cumulus Frontend UI on http://localhost:$(FRONTEND_PORT)..."
	cd web && npm run dev -- --port $(FRONTEND_PORT)

frontend: run-frontend
ui: run-frontend

# Starts backend & frontend together after starting Docker database
run: db-up
	@echo "=================================================="
	@echo "  Starting Cumulus Development Stack..."
	@echo "  Backend API:  http://localhost:$(HTTP_PORT)"
	@echo "  Frontend UI:  http://localhost:$(FRONTEND_PORT)"
	@echo "=================================================="
	@cd web && npm run dev -- --port $(FRONTEND_PORT) > /dev/null 2>&1 & FRONTEND_PID=$$!; \
	trap 'kill -9 $$FRONTEND_PID 2>/dev/null || true' EXIT INT TERM; \
	HTTP_PORT=$(HTTP_PORT) \
	DATABASE_URL=$(DATABASE_URL) \
	GOOGLE_SCOPES=$(GOOGLE_SCOPES) \
	ENCRYPTION_KEY=$(ENCRYPTION_KEY) \
	go run ./cmd/cumulus

dev: run

stop-backend:
	@echo "Stopping Cumulus backend (port $(HTTP_PORT))..."
	@pids="$$(lsof -t -i :$(HTTP_PORT) 2>/dev/null || true)"; \
	if [ -n "$$pids" ]; then \
		kill -9 $$pids 2>/dev/null || true; \
	fi; \
	pkill -x "cumulus" 2>/dev/null || true


stop-frontend:
	@echo "Stopping Cumulus frontend (port $(FRONTEND_PORT))..."
	@pids="$$(lsof -t -i :$(FRONTEND_PORT) 2>/dev/null || true)"; \
	if [ -n "$$pids" ]; then \
		kill -9 $$pids 2>/dev/null || true; \
	fi; \
	pkill -f "[h]ttp.server.*$(FRONTEND_PORT)" 2>/dev/null || true



# Stops backend, frontend, and PostgreSQL Docker container
stop: stop-backend stop-frontend db-stop

test:
	@echo "Running tests..."
	go test ./...

clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
