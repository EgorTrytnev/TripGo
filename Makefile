APP=trip-service
BIN=./bin/$(APP)

.PHONY: build run test tidy

build:
	@mkdir -p bin
	@go build -o $(BIN) ./cmd/$(APP)

run:
	@go run ./cmd/$(APP)

test:
	@go test ./...

tidy:
	@go mod tidy
