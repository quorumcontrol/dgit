gosources = $(shell find . -type f -name '*.go' -print)

FIRSTGOPATH = $(firstword $(subst :, ,$(GOPATH)))
HEAD_TAG := $(shell git tag --points-at HEAD)
GIT_REV := $(shell git rev-parse --short HEAD)
VERSION := $(or $(HEAD_TAG),$(GIT_REV))
DEV_VERSION := $(shell git diff-index --quiet HEAD || echo "${VERSION}-dev")
GOLDFLAGS += -X main.Version=$(or $(DEV_VERSION),$(VERSION))
GOFLAGS = -ldflags "$(GOLDFLAGS)"

all: build

dgit: go.mod go.sum $(gosources)
	go build -o dgit $(GOFLAGS) .

build: dgit

$(FIRSTGOPATH)/bin/dgit: dgit
	cp dgit $(FIRSTGOPATH)/bin/

$(FIRSTGOPATH)/bin/git-remote-dgit:
	cp git-remote-dgit $(FIRSTGOPATH)/bin/

dgit.tar.gz: dgit git-remote-dgit
	tar -czvf dgit.tar.gz dgit git-remote-dgit

tarball: dgit.tar.gz

install: $(FIRSTGOPATH)/bin/dgit $(FIRSTGOPATH)/bin/git-remote-dgit

test:
	go test ./...

clean:
	rm -f dgit dgit.tar.gz

.PHONY: all build tarball install test clean
