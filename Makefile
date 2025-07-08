.PHONY: test install fmt build

test:
	@go test ./...

install:
	@go install ./cmd/farm

fmt:
	@go fmt ./...

build:
	@./scripts/build.sh
