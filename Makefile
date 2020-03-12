gosources = $(shell find . -type f -name '*.go' -print)

FIRSTGOPATH = $(firstword $(subst :, ,$(GOPATH)))

all: build

dgit: go.mod go.sum $(gosources)
	go build -o dgit

build: dgit

$(FIRSTGOPATH)/bin/dgit: dgit
	cp dgit $(FIRSTGOPATH)/bin/

$(FIRSTGOPATH)/bin/git-remote-dgit:
	cp git-remote-dgit $(FIRSTGOPATH)/bin/

install: $(FIRSTGOPATH)/bin/dgit $(FIRSTGOPATH)/bin/git-remote-dgit

test:
	go test ./...

clean:
	rm dgit

.PHONY: all build install test clean
