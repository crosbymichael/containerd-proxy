PKG=github.com/crosbymichael/containerd-proxy
GO=$(shell which go)
GOFILES=$(shell find . -name "*.go")

SCOPE_LABEL=com.crosbymichael/containerd-proxy.scope
ANY_SCOPE=*
LDFLAGS=-ldflags '-s -w -X main.ScopeLabel="$(SCOPE_LABEL)" -X main.AnyScope="$(ANY_SCOPE)"'

all: build test

.PHONY: build
build: bin/containerd-proxy

.PHONY: clean
clean:
	$(RM) -r bin/

bin/containerd-proxy: $(GOFILES)
	$(GO) build $(LDFLAGS) -o $@ ./...

.PHONY: test
test: $(GOFILES)
	$(GO) test -v ./...
