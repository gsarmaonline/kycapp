.PHONY: test test-unit test-store test-web build run sqlc compose-up compose-down compose-logs

test:
	go test ./... -count=1 -timeout 5m
	$(MAKE) test-web

test-go:
	go test ./... -count=1 -timeout 5m

test-unit:
	go test ./internal/config/... ./internal/http/... ./internal/store/ -run 'TestMigrationFilesEmbedded|TestLoad|TestHealth|TestReady' -count=1

test-store:
	go test ./internal/store/ -count=1 -timeout 3m -v

test-web:
	cd web && npm test

sqlc:
	$$(go env GOPATH)/bin/sqlc generate

build:
	go build -o bin/api ./cmd/api

run: compose-up
	@echo "Open http://localhost:8080"

compose-up:
	docker compose up --build -d

compose-down:
	docker compose down

compose-logs:
	docker compose logs -f
