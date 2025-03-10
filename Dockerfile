FROM golang:1.11-alpine as maker

RUN set -eux; \
    apk add gcc \
        musl-dev

ADD . /usr/local/go/src/github.com/vitelabs/go-vite
RUN go build -o gvite  github.com/vitelabs/go-vite/cmd/gvite

FROM alpine:3.8
RUN apk update && \
	apk upgrade && \	
	apk add --no-cache bash bash-doc bash-completion ca-certificates && \
	rm -rf /var/cache/apk/* && \
	ln -s /root/.gvite /data && \
	/bin/bash

COPY --from=maker /go/gvite .
COPY ./node_config.json .
COPY ./docker-gvite .
EXPOSE 8483 8484 48132 41420 8483/udp
VOLUME ["/data"]
ENTRYPOINT ["/docker-gvite"] 
