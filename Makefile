.PHONY: install lint lint-fix test build

install:
	go mod download

lint:
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "The following files are not gofmt-formatted:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	golangci-lint run ./... --out-format=colored-line-number
	go mod tidy -diff

lint-fix:
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	go fmt ./...
	golangci-lint run --fix ./... --out-format=colored-line-number
	go mod tidy

test:
	go test -v -cover ./...

build:
	go build ./...
