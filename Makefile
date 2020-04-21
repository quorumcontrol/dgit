gosources = $(shell find . -type f -name '*.go' -print)

FIRSTGOPATH = $(firstword $(subst :, ,$(GOPATH)))
HEAD_TAG := $(shell if [ -d .git ]; then git tag --points-at HEAD; fi)
GIT_REV := $(shell if [ -d .git ]; then git rev-parse --short HEAD; fi)
GIT_VERSION := $(or $(HEAD_TAG),$(GIT_REV))
DEV_VERSION := $(shell if [ -d .git ]; then git diff-index --quiet HEAD || echo "${GIT_VERSION}-dev"; fi)
VERSION ?= $(or $(DEV_VERSION),$(GIT_VERSION))
GOLDFLAGS += -X main.Version=$(VERSION)
GOFLAGS = -ldflags "$(GOLDFLAGS)"

ifeq ($(PREFIX),)
	PREFIX := $(or $(FIRSTGOPATH),/usr/local)
endif

all: build

git-dg: go.mod go.sum $(gosources)
	go build -o $@ $(GOFLAGS) .

build: git-dg

dist/armv%/git-dg: go.mod go.sum $(gosources)
	mkdir -p $(@D)
	env GOOS=linux GOARCH=arm GOARM=$* go build -o $@ $(GOFLAGS)

dist/arm64v%/git-dg: go.mod go.sum $(gosources)
	mkdir -p $(@D)
	env GOOS=linux GOARCH=arm64 go build -o $@ $(GOFLAGS)

build-linux-arm: dist/armv6/git-dg dist/armv7/git-dg dist/arm64v8/git-dg

decentragit.tar.gz: git-dg git-remote-dg
	tar -czvf decentragit.tar.gz $^

dist/armv%/decentragit.tar.gz: dist/armv%/git-dg git-remote-dg
	tar -czvf $@ $^

dist/arm64v%/decentragit.tar.gz: dist/arm64v%/git-dg git-remote-dg
	tar -czvf $@ $^

tarball: decentragit.tar.gz

tarball-linux-arm: dist/armv6/decentragit.tar.gz dist/armv7/decentragit.tar.gz dist/arm64v8/decentragit.tar.gz

install: git-dg git-remote-dg
	install -d $(DESTDIR)$(PREFIX)/bin/
	install -m 755 git-dg $(DESTDIR)$(PREFIX)/bin/
	install -m 755 git-remote-dg $(DESTDIR)$(PREFIX)/bin/

uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/git-dg
	rm -f $(DESTDIR)$(PREFIX)/bin/git-remote-dg

test:
	go test ./...

clean:
	rm -f git-dg dgit.tar.gz

.PHONY: all build build-linux-arm tarball tarball-linux-arm install uninstall test clean
