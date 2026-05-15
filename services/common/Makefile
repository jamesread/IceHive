.PHONY: all build test lint clean

all: build

build:
	go build ./...

test:
	go test -v -race ./...

lint:
	go vet ./...

clean:
	go clean ./...
