.PHONY: build test mocks docker-image push-docker-image

default: build

sha := ${shell git rev-parse HEAD}

build:
	go install -ldflags '-X main.GitSHA=${sha}' github.com/dollarshaveclub/node-auto-repair-operator/cmd/node-auto-repair-operator

test:
	go test github.com/dollarshaveclub/node-auto-repair-operator/pkg/... \
		github.com/dollarshaveclub/node-auto-repair-operator/cmd/...

mocks:
	rm pkg/naro/testutil/mocks/*
	mockery -all -dir pkg/naro -output pkg/naro/testutil/mocks

docker-image:
	docker build -t quay.io/dollarshaveclub/node-auto-repair-operator:master .

push-docker-image:
	docker push quay.io/dollarshaveclub/node-auto-repair-operator:master
