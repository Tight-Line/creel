.PHONY: all build test vet lint proto-gen proto-lint clean run docker-build compose-up compose-down test-coverage test-coverage-check test-integration dev dev-down dev-migrate

all: proto-gen lint vet test build

build:
	go build -o bin/creel ./cmd/creel
	go build -o bin/creel-cli ./cmd/creel-cli
	go build -o bin/creel-chat ./cmd/creel-chat

test:
	go test -race -count=1 ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

proto-gen:
	buf generate
	go mod tidy

proto-lint:
	buf lint

run: build
	./bin/creel --config creel.yaml

docker-build:
	docker build -f deploy/docker/Dockerfile -t creel:local .

compose-up:
	docker compose up -d

compose-down:
	docker compose down

test-coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic -count=1 ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-coverage-check:
	./scripts/check-coverage.sh

# Run the full test + coverage suite against a local Postgres (same as CI).
# Starts the docker-compose postgres if it isn't already running.
# Uses a separate "creel_test" schema so it won't clobber local dev data.
# Migrations are run automatically by the integration test setup.
test-integration:
	@docker compose up -d postgres --wait
	CREEL_POSTGRES_HOST=localhost \
	CREEL_POSTGRES_PORT=5432 \
	CREEL_POSTGRES_USER=creel \
	CREEL_POSTGRES_PASSWORD=creel \
	CREEL_POSTGRES_NAME=creel \
	CREEL_POSTGRES_SCHEMA=creel_test \
	CREEL_POSTGRES_SSLMODE=disable \
	./scripts/check-coverage.sh

dev:
	docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build

dev-down:
	docker compose -f docker-compose.yml -f docker-compose.dev.yml down

dev-migrate:
	docker compose -f docker-compose.yml -f docker-compose.dev.yml run --rm migrate

clean:
	rm -rf bin/ coverage.out coverage.html coverage.filtered.out tmp/
