#!/bin/sh

CURRENT=$(cd "$(dirname "$0")" && pwd)
docker run --rm -it \
    -e GO111MODULE=on \
    -v "$CURRENT/.mod":/go/pkg/mod \
    -v "$CURRENT":/go/src/github.com/shogo82148/ssm-sign-proxy \
    -w /go/src/github.com/shogo82148/ssm-sign-proxy golang:1.12.1 "$@"
