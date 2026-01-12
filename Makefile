.PHONY: build run test clean dev sqlc docker-setup docker-start docker-stop docker-restart docker-logs docker-reset

BINARY=chat-empleados
DB_FILE=chat.db

# ========== Desarrollo local ==========

build:
	go build -o $(BINARY) .

run: build
	./$(BINARY)

dev:
	go run .

test:
	go test ./... -v

check:
	go build ./...
	go vet ./...

sqlc:
	sqlc generate

clean:
	rm -f $(BINARY)

reset-db:
	rm -f $(DB_FILE)
	sqlite3 $(DB_FILE) < schema.sql

# ========== Docker (produccion) ==========

docker-setup:
	./scripts/setup.sh

docker-start:
	./scripts/start.sh

docker-stop:
	./scripts/stop.sh

docker-restart:
	./scripts/restart.sh

docker-restart-rebuild:
	./scripts/restart.sh --rebuild

docker-logs:
	./scripts/logs.sh

docker-reset:
	./scripts/reset-db.sh

docker-build:
	docker compose build

docker-rebuild:
	docker compose build --no-cache

