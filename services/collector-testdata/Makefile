BINARY := collector-testdata

.PHONY: all build test lint clean run

all: build

build:
	go build -o $(BINARY) .

test:
	go test -v -race ./...

lint:
	go vet ./...

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)
	go clean ./...
