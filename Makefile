BINARY := poplar

build:
	go build -o $(BINARY) ./cmd/poplar

test:
	go test -tags=dev ./...

test-imap:
	go test -tags=integration ./internal/mailimap/...

vet:
	go vet ./...

fmt-check:
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then \
		echo "gofmt: $$out"; \
		echo "run: gofmt -w ."; \
		exit 1; \
	fi

lint:
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

audit:
	@mkdir -p docs/poplar/audits
	@echo "Running deadcode..."
	@deadcode ./cmd/poplar > docs/poplar/audits/2026-05-03-deadcode.txt 2>&1 || true
	@echo "Running unparam..."
	@unparam ./... > docs/poplar/audits/2026-05-03-unparam.txt 2>&1 || true
	@echo "Running golangci-lint..."
	@golangci-lint run ./... > docs/poplar/audits/2026-05-03-golangci.txt 2>&1 || true
	@echo "Audit outputs written to docs/poplar/audits/"

install:
	GOBIN=$(HOME)/.local/bin go install ./cmd/poplar

voice:
	@./scripts/voice-check.sh

modern-go-check:
	@./scripts/modern-go-check.sh

check: fmt-check vet voice modern-go-check test

clean:
	rm -f $(BINARY)

.PHONY: build test test-imap vet fmt-check voice modern-go-check lint audit install check clean
