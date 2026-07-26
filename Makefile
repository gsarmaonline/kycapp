.PHONY: test test-unit test-store test-web test-e2e build run sqlc openapi compose-up compose-down compose-logs web worker

test:
	go test ./... -count=1 -timeout 5m
	$(MAKE) test-web

test-go:
	go test ./... -count=1 -timeout 5m

test-unit:
	go test ./internal/config/... ./internal/http/... ./internal/store/ -run 'TestMigrationFilesEmbedded|TestLoad|TestHealth|TestReady' -count=1

test-store:
	go test ./internal/store/ -count=1 -timeout 3m -v

# Local API e2e (testcontainers Postgres + noop Stripe + recording mailer). Requires Docker.
test-e2e:
	go test ./internal/service/ -run 'TestE2ELocal' -count=1 -timeout 3m -v

test-web:
	cd web && npm test

sqlc:
	$$(go env GOPATH)/bin/sqlc generate

# Sync full OpenAPI to the web app and generate the merchant Integration subset.
openapi:
	cp docs/openapi.yaml web/public/openapi.yaml
	go run ./cmd/openapi-filter -in docs/openapi.yaml -out web/public/openapi-integration.yaml

build:
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker

worker:
	go run ./cmd/worker

run: compose-up
	@echo "Open http://localhost:8080"

compose-up:
	docker compose up --build -d

compose-down:
	docker compose down

compose-logs:
	docker compose logs -f

web:
	docker compose up --build -d web
	@echo "Open http://localhost:8080"
