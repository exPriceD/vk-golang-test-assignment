.PHONY: build run test clean deps

deps:
	go mod download
	go mod tidy

build: deps
	go build -o bin/worker-pool cmd/main.go

run: build
	./bin/worker-pool

test: deps
	go test -v ./...

clean:
	rm -rf bin/ 