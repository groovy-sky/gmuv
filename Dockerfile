FROM golang:alpine3.16

RUN apk add --no-cache git && go install github.com/groovy-sky/gmuv/v2@latest

