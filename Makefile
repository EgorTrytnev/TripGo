APP=trip-service
BIN=./bin/$(APP)

.PHONY: build run test test-integration lint tidy generate migrate migrate-down

build:
	@mkdir -p bin
	@go build -o $(BIN) ./cmd/$(APP)

run:
	@go run ./cmd/$(APP)

migrate:
	@set -a; . ./.env; set +a; go tool goose -dir migrations postgres "$$DATABASE_URL" up

migrate-down:
	@set -a; . ./.env; set +a; go tool goose -dir migrations postgres "$$DATABASE_URL" down-to 0

generate:
	@go tool oapi-codegen \
		-generate types,chi-server \
		-include-operation-ids createTrip,getTrip,finishTrip,health,ready \
		-package api \
		-o internal/generated/api.gen.go \
		contracts/openapi/trip-service.openapi.yaml
	@go generate ./internal/usecase/trip/...

test:
	@go test -race -coverprofile=/tmp/tripgo.cover ./...
	@go tool cover -func=/tmp/tripgo.cover | tail -n 1

test-integration:
	@set -a; . ./.env; set +a; go test -race -tags=integration ./internal/repository/...

lint:
	@golangci-lint run

tidy:
	@go mod tidy
