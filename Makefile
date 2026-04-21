.PHONY: build test lint clean tidy-check

BINARY = bin/ojs
GO ?= go
GO_ENV := GOWORK=off GOFLAGS=-mod=readonly

build:
	$(GO_ENV) $(GO) build -o $(BINARY) ./cmd/ojs/

test:
	$(GO_ENV) $(GO) test ./... -race -cover

lint:
	$(GO_ENV) $(GO) vet ./...

tidy-check:
	GOWORK=off $(GO) mod tidy -diff

clean:
	rm -rf bin/

run: build
	./$(BINARY) $(ARGS)
