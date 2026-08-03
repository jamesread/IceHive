BINARY := persister-yaml

.PHONY: all build test lint clean run

all: build

build:
	go build -o $(BINARY) .

test:
	go test -v -race ./...

lint:
	go fmt ./...
	go vet ./...
	golangci-lint run ./...

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)
	go clean ./...
