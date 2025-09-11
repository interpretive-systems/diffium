BIN ?= bin/diffium

.PHONY: build test run fmt

build:
	go build -o $(BIN) ./cmd/diffium

test:
	go test ./...

run:
	go run ./cmd/diffium watch

fmt:
	go fmt ./...

