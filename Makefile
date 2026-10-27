GO ?= go
BINARY ?= ytui

.PHONY: all build test vet check run clean

all: build

build:
	$(GO) build -o $(BINARY) .

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

check: test vet

run:
	$(GO) run .

clean:
	rm -f $(BINARY)
