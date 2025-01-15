GOOS=linux
GOARCH=amd64
VERSION := $(shell cat version.txt)
BINARY=sampleapp
.PHONY: build

GIT_COMMIT := $(shell git rev-list -1 HEAD)

build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY)-linux-x64 main.go
	GOOS=darwin GOARCH=$(GOARCH) go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY)-darwin-x64 main.go
