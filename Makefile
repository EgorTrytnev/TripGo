APP=trip-service
BIN=./bin/$(APP)

.PHONY: build run test tidy generate migrate migrate-down

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

test:
	@go test -race ./...

tidy:
	@go mod tidy
