gosources = $(shell find . -type f -name '*.go' -print)

FIRSTGOPATH = $(firstword $(subst :, ,$(GOPATH)))
HEAD_TAG := $(shell if [ -d .git ]; then git tag --points-at HEAD; fi)
GIT_REV := $(shell if [ -d .git ]; then git rev-parse --short HEAD; fi)
GIT_VERSION := $(or $(HEAD_TAG),$(GIT_REV))
DEV_VERSION := $(shell if [ -d .git ]; then git diff-index --quiet HEAD || echo "${GIT_VERSION}-dev"; fi)
VERSION ?= $(or $(DEV_VERSION),$(GIT_VERSION))
GOLDFLAGS += -X main.Version=$(VERSION)
GOFLAGS = -ldflags "$(GOLDFLAGS)"

all: build

dgit: go.mod go.sum $(gosources)
	go build -o dgit $(GOFLAGS) .

build: dgit

$(FIRSTGOPATH)/bin/dgit: dgit
	cp $< $(FIRSTGOPATH)/bin/$<

$(FIRSTGOPATH)/bin/git-remote-dgit: git-remote-dgit
	cp $< $(FIRSTGOPATH)/bin/$<

dgit.tar.gz: dgit git-remote-dgit
	tar -czvf dgit.tar.gz $^

tarball: dgit.tar.gz

install: $(FIRSTGOPATH)/bin/dgit $(FIRSTGOPATH)/bin/git-remote-dgit

uninstall:
	rm -f $(FIRSTGOPATH)/bin/dgit
	rm -f $(FIRSTGOPATH)/bin/git-remote-dgit

test:
	go test ./...

clean:
	rm -f dgit dgit.tar.gz

.PHONY: all build tarball install uninstall test clean
