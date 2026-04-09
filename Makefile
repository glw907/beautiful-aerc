build:
	go build -o mailrender ./cmd/mailrender
	go build -o pick-link ./cmd/pick-link
	go build -o fastmail-cli ./cmd/fastmail-cli
	go build -o tidytext ./cmd/tidytext
	go build -o compose-prep ./cmd/compose-prep

test:
	go test ./...

vet:
	go vet ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

install:
	GOBIN=$(HOME)/.local/bin go install ./cmd/mailrender
	GOBIN=$(HOME)/.local/bin go install ./cmd/pick-link
	GOBIN=$(HOME)/.local/bin go install ./cmd/fastmail-cli
	GOBIN=$(HOME)/.local/bin go install ./cmd/tidytext
	GOBIN=$(HOME)/.local/bin go install ./cmd/compose-prep

check: vet test

clean:
	rm -f mailrender pick-link fastmail-cli tidytext compose-prep

.PHONY: build test vet lint install check clean
