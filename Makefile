.PHONY: all test clean build install

all: install

default:
	go build ./...

build:
	go build ./...

install:
	go get ./...

clean:
	go clean -i ./...
