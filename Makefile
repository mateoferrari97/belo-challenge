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

.PHONY: compose-up
compose-up:
	@docker compose up --build

.PHONY: compose-down
compose-down:
	@docker compose down

.PHONY: compose-reset
compose-reset:
	@docker compose down -v

.PHONY: migrate-up
migrate-up:
	@migrate -path ./migrations -database "$(DATABASE_URL)" up

.PHONY: migrate-down
migrate-down:
	@migrate -path ./migrations -database "$(DATABASE_URL)" down 1

.PHONY: swag
swag:
	@echo "=> Generating OpenAPI spec (swag)"
	@go tool github.com/swaggo/swag/cmd/swag init \
		--generalInfo cmd/web/server/main.go \
		--dir ./ \
		--parseInternal \
		--parseDependency \
		--output docs/api \
		--packageName api \
		--generatedTime=false

.PHONY: swag-check
swag-check: swag
	@git diff --exit-code docs/api/ || (echo "docs/api is stale — run 'make swag' and commit" && exit 1)
