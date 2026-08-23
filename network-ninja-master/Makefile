SHELL := /usr/bin/env bash
VERSION ?= 0.2.0

.PHONY: fmt test vet build iso package clean

fmt:
	gofmt -w cmd/network-ninja/main.go internal/*/*.go

test:
	go test ./...

vet:
	go vet ./...

build:
	VERSION=$(VERSION) ./scripts/build.sh

iso: build
	VERSION=$(VERSION) ./scripts/build-iso.sh

package: build
	VERSION=$(VERSION) ./scripts/package-release.sh

clean:
	rm -rf dist/linux-amd64 build/live/.work
