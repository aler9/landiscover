#!/bin/sh

if [ -z "$IN_DOCKER" ]; then
    docker build . -t landiscover \
        && docker run -it --rm \
        -v $PWD:/src \
        landiscover $@
    exit $?
fi

build() {
    go build -v -o build_x64/landiscover \
        || exit 1

    # GOOS=linux \
    #     GOARCH=arm \
    #     GOARM=7 \
    #     CGO_ENABLED=1 \
    #     CC=arm-linux-gnueabihf-gcc \
    #     LD=arm-linux-gnueabihf-ld \
    #     go build -v -o build_arm7/landiscover \
    #     || exit 1
}

shell() {
    /bin/bash
}

case "$1" in
    "") build;;
    shell) shell;;
    *) echo "unrecognized command"; exit 1;;
esac
