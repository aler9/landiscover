
# landiscover

[![Release](https://img.shields.io/github/release/gswly/landiscover.svg)](https://github.com/gswly/landiscover/releases)

![](readme_assets/animated.gif)

Landiscover is a zero-configuration command-line tool that allows to discover every device connected to a local network, together with their hostname, in a very short period of time. It is intended for fast service discovery (i.e. finding a printer or a IoT device), without recurring to other tools that are often slow, difficult to remember or offer partial results. Although there are many applications already available for the scope, this one combines multiple techniques present individually in other softwares, in order to obtain the most complete result available without recurring to port scanning-based tool (i.e. Nmap). In particular:
* Arping technique is used for node discovery;
* DNS protocol is used for hostname discovery;
* Multicast DNS (MDNS) protocol is used for node and hostname discovery;
* NetBIOS protocol is used for node and hostname discovery.

The software is entirely written in Go, and the only external dependency is libpcap.


## Installation

Download, compile and install in your system with a single command:
```
docker run --rm -it \
    -v /usr/bin:/out \
    golang:1.11-stretch \
    bash -c "apt update && apt install -y libpcap-dev \
    && go get github.com/gswly/landiscover \
    && cp /go/bin/landiscover /out/"
```

Notes:
* Docker is required and is the only dependency
* Replace `/usr/bin` with the desired installation folder

## Usage

```
landiscover [--passive] [interface]
```
