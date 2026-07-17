.PHONY: test test-unit test-store build run compose-up compose-down

test:
	go test ./... -count=1 -timeout 3m

test-unit:
	go test ./internal/config/... ./internal/http/... ./internal/store/ -run 'TestMigrationFilesEmbedded|TestLoad|TestHealth|TestReady' -count=1

test-store:
	go test ./internal/store/ -count=1 -timeout 3m -v

build:
	go build -o bin/api ./cmd/api

run: compose-up
	DATABASE_URL='postgres://kyc:kyc@localhost:5432/kyc?sslmode=disable' go run ./cmd/api

compose-up:
	docker compose up -d

compose-down:
	docker compose down
