.PHONY: build test clean

build: bin/alcatraz-worker bin/spawn-client

bin/alcatraz-worker:
	go build -o bin/alcatraz-worker ./cmd/alcatraz-worker

bin/spawn-client:
	go build -o bin/spawn-client ./cmd/spawn-client

test:
	go test ./...

clean:
	rm -f bin/alcatraz-worker bin/spawn-client