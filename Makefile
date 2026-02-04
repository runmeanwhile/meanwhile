.PHONY: test lint gosec

test:
	go test ./...

lint:
	golangci-lint run
	gosec ./...

gosec:
	gosec ./...
