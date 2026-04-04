BINARY := beautiful-aerc

build:
	go build -o $(BINARY) ./cmd/beautiful-aerc

test:
	go test ./...

vet:
	go vet ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

install:
	go install ./cmd/beautiful-aerc

check: vet test

clean:
	rm -f $(BINARY)

.PHONY: build test vet lint install check clean
