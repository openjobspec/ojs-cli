.PHONY: build test lint clean

BINARY = bin/ojs

build:
	go build -o $(BINARY) ./cmd/ojs/

test:
	go test ./... -race -cover

lint:
	go vet ./...

clean:
	rm -rf bin/

run: build
	./$(BINARY) $(ARGS)
