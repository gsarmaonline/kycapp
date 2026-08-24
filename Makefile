.PHONY: test test-unit test-store test-web test-e2e test-sdk test-access build run sqlc openapi sdk sdk-go sdk-ts sdk-check compose-up compose-down compose-logs web worker

test:
	go test ./... -count=1 -timeout 5m
	$(MAKE) test-access
	$(MAKE) test-sdk
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

# core/access is a separate zero-dependency module (it compiles into the SDKs too),
# so the root `go test ./...` does not reach it.
test-access:
	cd core/access && go vet ./... && go test ./... -count=1 -race

# sdk/go is a separate module, so the root `go test ./...` does not reach it.
test-sdk:
	cd sdk/go && go build ./... && go vet ./...
	cd sdk/ts && npm run typecheck

sqlc:
	$$(go env GOPATH)/bin/sqlc generate

# Sync full OpenAPI to the web app and generate the merchant Integration subset.
# The authorisation schema is generated here too: it is a compile-time constant,
# so the docs render it from a file rather than asking the server about it.
openapi:
	cp docs/openapi.yaml web/public/openapi.yaml
	go run ./cmd/openapi-filter -in docs/openapi.yaml -out web/public/openapi-integration.yaml
	go run ./cmd/access-schema -out web/public/access-schema.json

# Regenerate the merchant SDK transport layers from the Integration spec.
# Both outputs are committed, so CI can detect drift with sdk-check.
sdk: openapi sdk-go sdk-ts

sdk-go:
	cd sdk/go && go tool oapi-codegen -config oapi-codegen.yaml ../../web/public/openapi-integration.yaml

sdk-ts:
	cd sdk/ts && npm run generate

# Fail when the committed spec or generated SDK code is stale. Used by CI.
sdk-check: sdk
	@git diff --exit-code -- web/public sdk/go/kyc/generated.go sdk/ts/src/generated \
		|| { echo; echo "Generated output is stale. Run 'make sdk' and commit the result."; exit 1; }

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
