.PHONY: build

default: build

build:
	go install github.com/dollarshaveclub/node-auto-repair-operator/pkg/cmd/node-auto-repair-operator

test:
	go test github.com/dollarshaveclub/node-auto-repair-operator/pkg/...
