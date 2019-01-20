
# landiscover

![](readme_assets/animated.gif)

Landiscover is a zero-configuration command-line tool that allows to discover every device connected to a local network within seconds. It is intended for fast service discovery (i.e. finding a printer or a IoT device), without recurring to other tools that are slow, difficult to remember or offer partial results. Although there are many tool already available for the scope, this one combines together multiple techniques, in order to obtain a complete output without running individual tools or recurring to more complex scanners (i.e. Nmap). In particular:
* Arping technique is used for node discovery;
* DNS protocol is used for hostname discovery;
* Multicast DNS (MDNS) protocol is used for node and hostname discovery;
* NetBIOS protocol is used for node and hostname discovery.

The software is entirely written in Go, and the only external dependency is libpcap.


## Installation

* Prebuilt binaries are available in the [release page](https://github.com/gswly/landiscover/releases).
* Otherwise it is possibile to build from source by following the instructions below.


## Usage

Open a terminal in the same directory as the executable and run:
```bash
./landiscover
```

It is also possible to set additional options by using the full syntax:

```bash
./landiscover [--passive] [interface]
```

## Compilation

Dependencies:
* libpcap headers
* go >= 1.11

Download required modules:
```bash
go mod init
```

Compile:
```
go build
```
