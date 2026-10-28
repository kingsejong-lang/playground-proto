# Usage: source a toolchain with buf, protoc-gen-{go,go-grpc,connect-go,es} on PATH, then:
#   make all        lint + breaking + generate + test
SHELL := /bin/bash
.PHONY: all lint breaking generate test ts-pack check-clean

all: lint generate test

lint:
	buf lint
	buf format --diff --exit-code

breaking:
	buf breaking --against '.git#branch=main'

generate:
	buf generate
	go mod tidy

test:
	go vet ./... && go test ./...
	cd ts && npm ci && npm run build && npm test

ts-pack:
	cd ts && npm ci && npm run build && npm pack

# CI: generated code must be committed
check-clean: generate
	git diff --exit-code -- gen ts/src/gen go.mod go.sum
