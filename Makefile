APP=trip-service
BIN=./bin/$(APP)

.PHONY: build run test tidy generate

build:
	@mkdir -p bin
	@go build -o $(BIN) ./cmd/$(APP)

run:
	@go run ./cmd/$(APP)

generate:
	@go tool oapi-codegen \
		-generate types,chi-server \
		-package api \
		-o internal/generated/api.gen.go \
		contracts/openapi/trip-service.openapi.yaml

test:
	@go test ./...

tidy:
	@go mod tidy
