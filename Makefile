VERSION = $(shell git describe --tags --always --abbrev | sed 's/^v//')

.PHONY: all tidy linux

all: linux

tidy:
	go mod tidy

linux: tidy
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=$(VERSION)" -o ./bin/plumber-linux-amd64 ./cmd
