.PHONY: build fix lint run web web-install generate test

generate:
	go generate ./...

web-install:
	cd frontend && ([ -d node_modules ] || npm ci)

web: web-install
	cd frontend && npm run build

build:
	go build -trimpath -ldflags="-s -w" ./...

# release: build frontend first, then the static Go binary
release: web
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/skills-fs .

fix:
	go fix ./...
	go mod tidy

run:
	go run .

lint:
	golangci-lint run ./...

test:
	go test ./...
