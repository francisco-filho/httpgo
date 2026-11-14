.PHONY: build fmt lint test e2e compatibility portable

build:
	go build -trimpath -o bin/httpgo ./cmd/httpgo

fmt:
	gofmt -s -w .
	goimports -w .

lint:
	golangci-lint run

test:
	go test ./... -race -cover

e2e:
	go test ./tests/e2e -count=1

# Explicit dependency on Hurl 8.0.0; missing/wrong version fails the test.
compatibility:
	go test -tags compatibility ./tests/e2e -run '^TestCompatibility$$' -count=1 -v

# Build evidence only, not evidence of native execution or release artifacts.
portable:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o bin/portable/httpgo-linux-amd64 ./cmd/httpgo
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -o bin/portable/httpgo-darwin-amd64 ./cmd/httpgo
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -o bin/portable/httpgo-windows-amd64.exe ./cmd/httpgo
