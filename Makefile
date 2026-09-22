BINARY := ./bin/logbook
MAIN := ./cmd/logbook
WITH_ENV = set -a; if [ -f .env ]; then . ./.env; fi; set +a;

.DEFAULT_GOAL := help
.PHONY: help build run test test-integration lint fmt migrate-up migrate-down user-create docker-up docker-down docker-logs docker-migrate

help:
	@printf '%s\n' 'make build | run | test | test-integration | lint | fmt' 'make migrate-up | migrate-down | user-create USERNAME=admin NAME="Admin" ROLE=admin' 'make docker-up | docker-down | docker-logs | docker-migrate'

build:
	go build -trimpath -ldflags="-s -w" -o $(BINARY) $(MAIN)

run:
	@$(WITH_ENV) go run $(MAIN) serve

test:
	go test -race ./...

test-integration:
	@test -n "$$TEST_DATABASE_URL" || (echo 'Set TEST_DATABASE_URL to a test PostgreSQL database'; exit 1)
	go test -race -count=1 ./...

lint:
	go vet ./...

fmt:
	gofmt -w cmd internal migrations

migrate-up:
	@$(WITH_ENV) go run $(MAIN) migrate up

migrate-down:
	@$(WITH_ENV) go run $(MAIN) migrate down

user-create:
	@$(WITH_ENV) go run $(MAIN) user create --username "$(USERNAME)" --name "$(NAME)" --role "$(or $(ROLE),operator)"

docker-up:
	docker compose up --build -d --wait

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f app

docker-migrate:
	docker compose run --rm migrate
