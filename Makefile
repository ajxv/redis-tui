.PHONY: build run test fmt lint lint-optional clean

BINARY_NAME=redis-tui
MAIN_PATH=./cmd/redis-tui
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  = -s -w -X main.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) $(MAIN_PATH)

run: build
	./$(BINARY_NAME)

test:
	go test -v ./...

fmt:
	go fmt ./...

lint:
	golangci-lint run ./...

lint-optional:
	golangci-lint run ./... || echo "Install golangci-lint: https://golangci-lint.run/usage/install/"

clean:
	go clean
	rm -f $(BINARY_NAME)
