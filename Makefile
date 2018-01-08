.PHONY: build test docker-image

default: build

build:
	go install github.com/dollarshaveclub/node-auto-repair-operator/pkg/cmd/node-auto-repair-operator

test:
	go test github.com/dollarshaveclub/node-auto-repair-operator/pkg/...

docker-image:
	docker build -t quay.io/dollarshaveclub/node-auto-repair-operator:master .

push-docker-image:
	docker push quay.io/dollarshaveclub/node-auto-repair-operator:master
