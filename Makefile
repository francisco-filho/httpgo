.PHONY: build fmt lint test

build:
	go build -trimpath -o bin/httpgo ./cmd/httpgo

fmt:
	gofmt -s -w .
	goimports -w .

lint:
	golangci-lint run

test:
	go test ./... -race -cover
