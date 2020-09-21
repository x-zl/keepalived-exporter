FROM golang:alpine

RUN apk add --no-cache make git bash

WORKDIR /keepalived-exporter

COPY . .

RUN make build

EXPOSE 9105

ENTRYPOINT [ "./keepalived-exporter" ]
