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
	buf generate
	go mod tidy

proto-lint:
	buf lint

clean:
	rm -rf bin/
