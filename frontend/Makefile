.PHONY: all build test lint clean dev

all: build

build:
	npm ci --silent
	npm run build

test:
	npm run build

lint:
	npx vue-tsc --noEmit
	npm run lint

dev:
	npm run dev

clean:
	rm -rf dist
