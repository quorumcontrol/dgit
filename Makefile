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

dist/armv%/dgit: go.mod go.sum $(gosources)
	mkdir -p $(@D)
	env GOOS=linux GOARCH=arm GOARM=$* go build -o $@

dist/arm64v%/dgit: go.mod go.sum $(gosources)
	mkdir -p $(@D)
	env GOOS=linux GOARCH=arm64 go build -o $@

build-linux-arm: dist/armv6/dgit dist/armv7/dgit dist/arm64v8/dgit

$(FIRSTGOPATH)/bin/dgit: dgit
	cp $< $(FIRSTGOPATH)/bin/$<

$(FIRSTGOPATH)/bin/git-remote-dgit: git-remote-dgit
	cp $< $(FIRSTGOPATH)/bin/$<

dgit.tar.gz: dgit git-remote-dgit
	tar -czvf dgit.tar.gz $^

dist/armv%/dgit.tar.gz: dist/armv%/dgit git-remote-dgit
	tar -czvf $@ $^

dist/arm64v%/dgit.tar.gz: dist/arm64v%/dgit git-remote-dgit
	tar -czvf $@ $^

tarball: dgit.tar.gz

tarball-linux-arm: dist/armv6/dgit.tar.gz dist/armv7/dgit.tar.gz dist/arm64v8/dgit.tar.gz

install: $(FIRSTGOPATH)/bin/dgit $(FIRSTGOPATH)/bin/git-remote-dgit

uninstall:
	rm -f $(FIRSTGOPATH)/bin/dgit
	rm -f $(FIRSTGOPATH)/bin/git-remote-dgit

test:
	go test ./...

clean:
	rm -f dgit dgit.tar.gz

.PHONY: all build build-linux-arm tarball tarball-linux-arm install uninstall test clean
