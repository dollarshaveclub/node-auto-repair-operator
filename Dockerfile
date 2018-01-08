FROM golang:1.9.2-alpine3.7
COPY . /go/src/github.com/dollarshaveclub/node-auto-repair-operator
WORKDIR /go/src/github.com/dollarshaveclub/node-auto-repair-operator
RUN apk add -U make git
RUN make

FROM alpine:3.7
WORKDIR /bin
COPY --from=0 /go/bin/node-auto-repair-operator .
CMD ["./app"]
ENTRYPOINT ["/bin/node-auto-repair-operator"]
CMD [""]
