# poplar's quality gate. One tier locally: a check either gates
# here or does not exist (build-machine design, section 2). Tool
# binaries build from tools/go.mod, an isolated module so the
# product go.mod stays dependency-free of the tooling; see
# docs/superpowers/specs/2026-07-27-poplar-build-machine.md.

BINARY := poplar
GOLANGCI := tools/bin/golangci-lint
POPLARCHECK := tools/bin/poplarcheck

.PHONY: all build test install fmt check conformance \
	tidy-check check-build fmt-check lint analyzers jmap-boundary tagged-vet vale-comments skipcheck hookcheck perf \
	build-golangci-lint build-poplarcheck gallery tmux-check

# The conformance suite's server. Podman leads because this is the
# runtime the target is actually exercised against, and because
# running a third-party mail server rootless is the safer default when
# a machine has both. CONTAINER=docker picks the other one
# deliberately. The host port is high because rootless podman cannot
# bind below 1024, so a low port would work under docker and fail here.
CONTAINER ?= $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)
CONFORMANCE_IMAGE := docker.io/stalwartlabs/stalwart:v0.16.15
CONFORMANCE_NAME := poplar-jmap-conformance
CONFORMANCE_PORT := 19080
CONFORMANCE_URL := http://localhost:$(CONFORMANCE_PORT)

all: check

build:
	go build -o $(BINARY) ./cmd/poplar

test:
	go test ./...

# gallery regenerates internal/ui/testdata/gallery/ (design decision
# 10, pass 2 amendment B): the render seam's own committed sweep of
# every fixture × profile × size point, the review medium the design
# language's prototyping loop runs on instead of an HTML mock. Plain
# `go test ./internal/ui/...`, part of the test step above, already
# fails on any drift (or an orphan file the current sweep no longer
# produces) between a fresh sweep and these files; this target reruns
# the same sweep with -update to accept a deliberate change. The
# -run pattern is anchored to TestGallery exactly, not a prefix
# match, so it does not also pull in TestGallery_TwoSweepsByteIdentical
# (a determinism check, not a file writer, that -update would not
# change the behavior of anyway, but running it here is pure waste).
# Scoped to the internal/ui package alone, not ./..., since -update
# is a flag internal/ui/fixtures' own test binary does not register.
#
# Churn policy (task 12, survey amendment G): this target is the only
# way a committed render or its ground-map sidecar changes. Read the
# diff before committing it, the same as any other golden;
# `git add -p` over testdata/gallery is the fast way to confirm every
# changed file traces to the screen a commit actually touched. A
# chrome task's own churn stays inside the screens it touched; a diff
# that also moves an unrelated fixture's render is a bug in the task,
# not a gallery to wave through. The pass-end reviewer reads gallery
# churn explicitly for exactly this reason.
gallery:
	go test ./internal/ui/ -run '^TestGallery$$' -update -count=1

install: build
	install -m 0755 $(BINARY) "$(HOME)/.local/bin/$(BINARY)"

fmt: build-golangci-lint
	./$(GOLANGCI) fmt --config .golangci.yml ./...

check: tidy-check check-build fmt-check lint analyzers jmap-boundary tagged-vet vale-comments skipcheck hookcheck test perf

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

# The analyzer tests run uncached: writecall's write-surface test
# type-checks internal/store, which lives outside the tools module,
# and the go test cache does not track files outside a test's own
# module (cmd/go skips them). A cached pass over a store that has
# since grown an export is the decay that test exists to catch.
analyzers: build-poplarcheck
	./$(POPLARCHECK) ./...
	go -C tools test -count=1 ./...

# jmap ships as a standalone library, so importing a poplar package
# would end its portability and, because it cannot reach
# internal/uerr, its promise to log nothing. The multichecker cannot
# see this: pkgrole classifies by an internal/ or cmd/ segment and
# jmap has neither, so all four analyzers skip it. The dependency
# list is the gate instead, over the test files too, since a fixture
# helper reaching into poplar travels with the package, and over
# ./jmap/... rather than ./jmap so a package added beside it is
# covered on the day it appears.
#
# Once per tag set, because go list reports the imports of the files
# the tags select and nothing else. The two server suites are the
# files most likely to reach for a poplar helper, since they are the
# ones with a whole application's worth of context around them.
JMAP_TAGS := "" conformance live

jmap-boundary:
	@bad=$$( for tags in $(JMAP_TAGS); do \
	          go list -tags "$$tags" -deps ./jmap/...; \
	          go list -tags "$$tags" -f '{{join .TestImports "\n"}}{{"\n"}}{{join .XTestImports "\n"}}' ./jmap/...; \
	        done \
	        | grep '^github.com/glw907/poplar' \
	        | grep -Ev '^github.com/glw907/poplar/jmap(/|$$)' | sort -u ); \
	if [ -n "$$bad" ]; then \
	  echo "jmap must import no poplar package, found:" >&2; \
	  echo "$$bad" >&2; exit 1; \
	fi

# The build-tagged suites compile under no other gate: lint, the
# analyzers and go test all read the default tags, so a conformance
# file that stopped compiling would be found by whoever next ran the
# container, which is by hand. Vet type-checks them here instead.
tagged-vet:
	go vet -tags "conformance live" ./jmap/...
	go vet -tags perf ./cmd/poplar/... ./internal/store/...

# The second-server validation, run by hand and never by check: check
# must pass on a machine with no container runtime at all. Stalwart
# boots into a setup mode that serves nothing but its own
# configuration object, so the restart between the two provisioning
# steps is the server leaving that mode. Teardown takes the anonymous
# volumes with it, so a rerun starts from an empty account, and it runs
# on the way out of a failed run too.
#
# The account credentials come out of the provisioner that created the
# account, rather than being written here as well. POPLAR_JMAP_REQUIRED
# is what turns the suite's missing-server skip into a failure: without
# it, a renamed variable leaves this target starting a container,
# provisioning it, skipping every test and reporting ok.
conformance:
	@test -n "$(CONTAINER)" || { echo "conformance needs podman or docker on PATH; found neither" >&2; exit 1; }
	@set -e; \
	$(CONTAINER) rm -f -v $(CONFORMANCE_NAME) >/dev/null 2>&1 || true; \
	trap '$(CONTAINER) rm -f -v $(CONFORMANCE_NAME) >/dev/null 2>&1' EXIT; \
	$(CONTAINER) run -d --name $(CONFORMANCE_NAME) \
		-p $(CONFORMANCE_PORT):8080 \
		-e STALWART_RECOVERY_ADMIN=admin:conformance \
		-e STALWART_PUBLIC_URL=$(CONFORMANCE_URL) \
		$(CONFORMANCE_IMAGE) >/dev/null; \
	go run ./scripts/conformance -step setup -url $(CONFORMANCE_URL); \
	$(CONTAINER) restart $(CONFORMANCE_NAME) >/dev/null; \
	go run ./scripts/conformance -step account -url $(CONFORMANCE_URL); \
	env $$(go run ./scripts/conformance -step env) \
		POPLAR_JMAP_SESSION_URL=$(CONFORMANCE_URL)/.well-known/jmap \
		POPLAR_JMAP_SERVER=stalwart \
		POPLAR_JMAP_REQUIRED=1 \
		go test -tags conformance -count=1 ./jmap/...

vale-comments:
	./scripts/vale-comments.sh

skipcheck:
	go run ./scripts/skipcheck

hookcheck:
	go run ./scripts/hookcheck

# The perf tag is what keeps the QA-1/2/3 certification tests out of
# the `test` step above, whose parallel package scheduling loads the
# machine enough to fail a latency gate the product passes: QA-1's
# single-sample cold start measured 907ms against its 500ms budget
# there, and 26-42ms alone. -p 1 completes the isolation by running the
# two perf packages one after the other rather than together.
#
# Race instrumentation costs 2-20x time and 5-10x memory, so a p95
# asserted under it measures the detector, not the product; race
# coverage runs in CI instead (section 2).
perf:
	go test -tags perf -p 1 -run 'QA[123]' -count=1 ./...

# tmux-check drives the built binary through the tier-3 smoke flow
# (task 12, survey amendment C): launch, switch all four surfaces,
# open help, quit, keyboard only, poplar's own exit code captured. It
# needs a real terminal and a real configured account, so it is a
# gate-platform tool run by hand at a pass gate, never part of `make
# check` or CI.
tmux-check: build
	./scripts/tmux-check

build-golangci-lint:
	go build -C tools -o bin/golangci-lint github.com/golangci/golangci-lint/v2/cmd/golangci-lint

build-poplarcheck:
	go build -C tools -o bin/poplarcheck ./analyzers/poplarcheck
