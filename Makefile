-include .envrc
export

%:
	@:

.PHONY: run
run:
	@go run ./cmd/api

.PHONY: build
build:
	@mkdir -p bin
	@go build -o bin/erawan-cluster ./cmd/api

.PHONY: test
test:
	@go test ./...

.PHONY: fmt
fmt:
	@gofmt -w cmd internal

.PHONY: tidy
tidy:
	@go mod tidy
