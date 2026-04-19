.PHONY: all
all: ensure-deps fmt lint test

.PHONY: ensure-deps
ensure-deps:
	@echo "=> Syncing dependencies with go mod tidy"
	@go mod tidy

.PHONY: fmt
fmt:
	@echo "=> Formatting (gofumpt + goimports)"
	@go tool golangci-lint fmt ./...

.PHONY: lint
lint:
	@echo "=> Linting"
	@go tool golangci-lint run ./...

.PHONY: test
test:
	@echo "=> Running tests"
	@go test ./... -covermode=atomic -coverpkg=./... -count=1 -race

.PHONY: run
run:
	@go run cmd/web/server/main.go
