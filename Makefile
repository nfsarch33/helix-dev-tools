VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
# Windows GUI subsystem: avoids allocating a console when Cursor spawns hook subprocesses (fewer flashes).
# Stdio pipes from the parent still work; use for hooks only — interactive CLI may show no console.
WINDOWS_GUI_LDFLAGS := $(LDFLAGS) -H windowsgui
BINARY         := helix-dev-tools
GOFLAGS := -race

HOST_OS   := $(shell uname -s | tr '[:upper:]' '[:lower:]')
HOST_ARCH := $(shell uname -m)
ifeq ($(HOST_ARCH),x86_64)
  HOST_GOARCH := amd64
else ifeq ($(HOST_ARCH),aarch64)
  HOST_GOARCH := arm64
else ifeq ($(HOST_ARCH),arm64)
  HOST_GOARCH := arm64
else
  HOST_GOARCH := $(HOST_ARCH)
endif

.PHONY: build build-zd-claude-proxy install-zd-claude-proxy test test-cover lint vuln security fuzz install dist-install docker docker-native test-docker release clean

build:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o bin/$(BINARY) ./cmd/helix-dev-tools/

build-zd-claude-proxy:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o bin/zd-claude-proxy ./cmd/zd-claude-proxy/

install-zd-claude-proxy: build-zd-claude-proxy
	@tmp=~/bin/.zd-claude-proxy.$$$$.new; \
	cp bin/zd-claude-proxy "$$tmp" && mv -f "$$tmp" ~/bin/zd-claude-proxy
	@echo "Installed to ~/bin/zd-claude-proxy"

test:
	go test $(GOFLAGS) -count=1 ./internal/...

test-cover:
	go test $(GOFLAGS) -count=1 -coverprofile=coverage.out ./internal/...
	go tool cover -func=coverage.out

lint:
	go vet ./...
	golangci-lint run --timeout 2m

vuln:
	govulncheck ./...

security: lint vuln
	gosec -severity high -quiet ./...
	@echo "Security scan complete"

fuzz:
	go test -fuzz=FuzzPatternMatcher -fuzztime=30s ./internal/patterns/

install: build
	@for dir in "$(HOME)/bin" "$(HOME)/.local/bin"; do \
		mkdir -p "$$dir"; \
		tmp="$$dir/.$(BINARY).$$$$.new"; \
		cp bin/$(BINARY) "$$tmp" && mv -f "$$tmp" "$$dir/$(BINARY)"; \
		echo "Installed to $$dir/$(BINARY)"; \
	done

install-compat: install
	@for dir in "$(HOME)/bin" "$(HOME)/.local/bin"; do \
		if [ ! -e "$$dir/cursor-tools" ]; then \
			ln -s "$$dir/$(BINARY)" "$$dir/cursor-tools"; \
			echo "Created backward-compat symlink $$dir/cursor-tools -> $(BINARY)"; \
		fi; \
	done

dist-install:
	@DIST=dist/$(BINARY)-$(HOST_OS)-$(HOST_GOARCH); \
	if [ -f "$$DIST" ]; then cp "$$DIST" ~/bin/$(BINARY) && echo "Installed from dist: $$DIST"; \
	else echo "No dist binary for $(HOST_OS)-$(HOST_GOARCH) -- run make install to build locally"; fi

docker:
	docker buildx build \
	  --platform linux/amd64,linux/arm64 \
	  --build-arg VERSION=$(VERSION) \
	  -f build/package/Dockerfile \
	  -t $(BINARY):$(VERSION) \
	  .

docker-native:
	docker build \
	  --build-arg VERSION=$(VERSION) \
	  -f build/package/Dockerfile \
	  -t $(BINARY):$(VERSION) \
	  .

test-docker:
	docker build -f build/package/Dockerfile.dev -t $(BINARY)-dev:latest .
	docker run --rm $(BINARY)-dev:latest

release:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64 ./cmd/helix-dev-tools/
	CGO_ENABLED=0 GOOS=linux  GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 ./cmd/helix-dev-tools/
	CGO_ENABLED=0 GOOS=linux  GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64 ./cmd/helix-dev-tools/
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe ./cmd/helix-dev-tools/
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="$(WINDOWS_GUI_LDFLAGS)" -o dist/$(BINARY)-windows-amd64-noconsole.exe ./cmd/helix-dev-tools/
	@echo "Built: darwin-arm64, linux-amd64, linux-arm64, windows-amd64.exe, windows-amd64-noconsole.exe"

clean:
	rm -rf bin/ coverage.out
