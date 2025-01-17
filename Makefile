.PHONY: all tidy linux

all: linux

tidy:
	go mod tidy

linux: tidy
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=$(shell git describe --tags --always --abbrev)" -o ./bin/plumber-linux-amd64 ./cmd
