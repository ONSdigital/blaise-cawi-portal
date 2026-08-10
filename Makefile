.PHONY: install lint lint-fix test build

install:
	go mod download

lint:
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	golangci-lint run ./... --out-format=colored-line-number
	go mod tidy -diff
	@unformatted=$$(gofmt -l $$(find . -name '*.go')); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt: unformatted files:\n$$unformatted"; \
		exit 1; \
	fi

lint-fix:
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	golangci-lint run --fix ./... --out-format=colored-line-number
	go mod tidy
	gofmt -w $$(find . -name '*.go')

test:
	go test -v -cover ./...

build:
	go build ./...
