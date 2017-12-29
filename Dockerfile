FROM golang:1.9.2-alpine3.7
COPY . /go/src/github.com/dollarshaveclub/node-auto-repair-operator
RUN go install github.com/dollarshaveclub/node-auto-repair-operator/pkg/cmd/node-auto-repair-operator

FROM alpine:3.7
WORKDIR /bin
COPY --from=0 /go/bin/node-auto-repair-operator .
# RUN apk --update add ca-certificates
CMD ["./app"]
ENTRYPOINT ["/bin/node-auto-repair-operator"]
CMD [""]
