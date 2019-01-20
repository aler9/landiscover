FROM amd64/golang:1.11-stretch

ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    git \
    g++ \
    libpcap-dev \
    libpcap0.8 \
    && rm -rf /var/lib/apt/lists/*

RUN dpkg --add-architecture armhf \
    && apt-get update && apt-get install -y --no-install-recommends \
    g++-arm-linux-gnueabihf \
    libpcap0.8:armhf \
    && rm -rf /var/lib/apt/lists/* \
    && ln -s /usr/lib/arm-linux-gnueabihf/libpcap.so.0.8 /usr/lib/arm-linux-gnueabihf/libpcap.so

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

ENV IN_DOCKER 1

ENTRYPOINT [ "./build.sh" ]
