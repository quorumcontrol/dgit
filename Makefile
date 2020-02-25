gosources = $(shell find . -type f -name '*.go' -print)

FIRSTGOPATH = $(firstword $(subst :, ,$(GOPATH)))

all: build

git-remote-dgit: go.mod go.sum $(gosources)
	go build -o git-remote-dgit

build: git-remote-dgit

$(FIRSTGOPATH)/bin/git-remote-dgit: git-remote-dgit
	cp git-remote-dgit $(FIRSTGOPATH)/bin/

install: $(FIRSTGOPATH)/bin/git-remote-dgit

clean:
	rm git-remote-dgit

PHONY: all build install clean