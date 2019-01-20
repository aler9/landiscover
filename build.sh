#!/bin/sh

if [ -z "$IN_DOCKER" ]; then
    docker build . -t landiscover \
        && docker run -it --rm \
        -v $PWD:/src \
        landiscover $@
    exit $?
fi

build_amd64() {
    go build -v -o build_amd64/landiscover
}

build_armv7() {
    GOOS=linux \
        GOARCH=arm \
        GOARM=7 \
        CGO_ENABLED=1 \
        CC=arm-linux-gnueabihf-gcc \
        LD=arm-linux-gnueabihf-ld \
        go build -v -o build_armv7/landiscover
}

shell() {
    /bin/bash
}

case "$1" in
    amd64) build_amd64;;
    armv7) build_armv7;;
    shell) shell;;
    *) echo "usage: $0 [amd64|armv7|shell]"; exit 1;;
esac
