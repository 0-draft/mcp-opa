BIN := mcp-opa-authz
PKG := ./...
IMAGE := ghcr.io/kanywst/$(BIN)
VERSION ?= dev

.PHONY: all build test cover vet lint smoke docker fmt tidy clean check

# What CI runs, in the order that fails fastest.
check: fmt vet lint test smoke

all: check

build:
	go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(BIN) .

test:
	go test -race -count=1 -v $(PKG)

cover:
	go test -race -count=1 -covermode=atomic -coverprofile=coverage.out $(PKG)
	go tool cover -func=coverage.out | tail -n 1

vet:
	go vet $(PKG)

lint:
	golangci-lint run $(PKG)

# Drives the real binary through one MCP stdio session against a fake PDP.
# Needs jq and python3.
smoke: build
	./scripts/smoke.sh ./$(BIN)

docker:
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE):$(VERSION) .

fmt:
	gofmt -s -w .

tidy:
	go mod tidy

clean:
	rm -f $(BIN) coverage.out
	rm -rf dist/
