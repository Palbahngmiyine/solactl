.PHONY: build test test-race lint clean fuzz

build:
	go build -o bin/solactl ./cmd/solactl

test:
	go test ./... -count=1

test-race:
	go test ./... -race -count=1

fuzz:
	go test ./pkg/validation/... -fuzz=. -fuzztime=10s

lint:
	go vet ./...

clean:
	rm -rf bin/
