GOOS=linux
GOARCH=amd64
VERSION=1.0.0
BINARY=sampleapp
.PHONY: build

GIT_COMMIT := $(shell git rev-list -1 HEAD)

build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(BINARY)-linux-x64 main.go
	GOOS=darwin GOARCH=$(GOARCH) go build -o $(BINARY)-darwin-x64 main.go
