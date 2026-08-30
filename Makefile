.PHONY: build test run-tui clean

build:
	go build -o bin/r19dev ./cmd/r19dev

test:
	go test -v ./...

run-tui: build
	./bin/r19dev tui /Volumes/home/BT/2026

clean:
	rm -rf bin/
