GO ?= go

.PHONY: test
test:
	$(GO) test ./... -race -count=1

.PHONY: cover
cover:
	$(GO) test ./... -coverprofile=coverage.out
	$(GO) tool cover -func=coverage.out | tail -1

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: lint
lint:
	golangci-lint run

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: build
build:
	$(GO) build -o bin/bot ./cmd/bot

.PHONY: up
up:
	docker compose up -d db

.PHONY: down
down:
	docker compose down

.PHONY: psql
psql:
	docker compose exec db psql -U chekmate

.PHONY: reset
reset:
	docker compose down -v
	docker compose up -d db
