# LR #1: Git/CI — Makefile for local development
# LR #11: Containerization

.PHONY: build run lint test gosec clean

build:
	go build -o bin/go-escrow .

run:
	go run .

lint:
	golangci-lint run ./...

test:
	go test ./... -v -coverprofile=cover.out

gosec:
	gosec ./...

clean:
	rm -rf bin/ cover.out
