.PHONY: all build test vet lint proto-gen proto-lint clean

all: lint vet test build

build:
	go build -o bin/creel ./cmd/creel
	go build -o bin/creel-cli ./cmd/creel-cli

test:
	go test -race -count=1 ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

proto-gen:
	@echo "proto-gen: not yet configured (see Step 1)"

proto-lint:
	@echo "proto-lint: not yet configured (see Step 1)"

clean:
	rm -rf bin/
