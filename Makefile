VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
BINARY  := cursor-tools
GOFLAGS := -race

.PHONY: build test test-all lint install docker release clean

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o bin/$(BINARY) ./cmd/cursor-tools/

test:
	go test $(GOFLAGS) -count=1 ./internal/...

test-cover:
	go test $(GOFLAGS) -count=1 -coverprofile=coverage.out ./internal/...
	go tool cover -func=coverage.out

lint:
	go vet ./...

install: build
	cp bin/$(BINARY) ~/bin/$(BINARY)
	@echo "Installed to ~/bin/$(BINARY)"

docker:
	docker build -f build/package/Dockerfile -t $(BINARY):$(VERSION) .

release:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o bin/$(BINARY)-darwin-arm64 ./cmd/cursor-tools/
	CGO_ENABLED=0 GOOS=linux  GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o bin/$(BINARY)-linux-amd64  ./cmd/cursor-tools/
	@echo "Built: bin/$(BINARY)-darwin-arm64, bin/$(BINARY)-linux-amd64"

clean:
	rm -rf bin/ coverage.out
