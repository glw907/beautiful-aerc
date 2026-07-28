# poplar's quality gate. One tier locally: a check either gates
# here or does not exist (build-machine design, section 2). Tool
# binaries build from tools/go.mod, an isolated module so the
# product go.mod stays dependency-free of the tooling; see
# docs/superpowers/specs/2026-07-27-poplar-build-machine.md.

BINARY := poplar
GOLANGCI := tools/bin/golangci-lint
POPLARCHECK := tools/bin/poplarcheck

.PHONY: all build test install fmt check \
	tidy-check check-build fmt-check lint analyzers vale-comments skipcheck perf \
	build-golangci-lint build-poplarcheck

all: check

build:
	go build -o $(BINARY) ./cmd/poplar

test:
	go test ./...

install: build
	install -m 0755 $(BINARY) "$(HOME)/.local/bin/$(BINARY)"

fmt: build-golangci-lint
	./$(GOLANGCI) fmt --config .golangci.yml ./...

check: tidy-check check-build fmt-check lint analyzers vale-comments skipcheck test perf

tidy-check:
	go mod tidy
	git diff --exit-code -- go.mod go.sum

# go build ./... misses main packages under test compilation, and the
# project promises a macOS build (C10), so both run here rather than
# only under `go test`. Output goes to a scratch directory, cleaned
# up after, so a lone main package (a case this module already has)
# doesn't leave a binary in the tree.
check-build:
	d="$$(mktemp -d)"; trap 'rm -rf "$$d"' EXIT; \
	go build -o "$$d/" ./... && \
	GOOS=darwin GOARCH=arm64 go build -o "$$d/" ./...

fmt-check: build-golangci-lint
	./$(GOLANGCI) fmt --config .golangci.yml --diff ./...

lint: build-golangci-lint
	./$(GOLANGCI) run --config .golangci.yml ./...

analyzers: build-poplarcheck
	./$(POPLARCHECK) ./...
	go -C tools test ./...

vale-comments:
	./scripts/vale-comments.sh

skipcheck:
	go run ./scripts/skipcheck

# Race instrumentation costs 2-20x time and 5-10x memory, so a p95
# asserted under it measures the detector, not the product; race
# coverage runs in CI instead (section 2).
perf:
	go test -run 'QA[123]' -count=1 ./...

build-golangci-lint:
	go build -C tools -o bin/golangci-lint github.com/golangci/golangci-lint/v2/cmd/golangci-lint

build-poplarcheck:
	go build -C tools -o bin/poplarcheck ./analyzers/poplarcheck
