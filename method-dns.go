package main

import (
	"net"
)

func (p *program) dnsRequest(key nodeKey, destIp net.IP) {
	names, err := net.LookupAddr(destIp.String())
	if err != nil {
		return
	}

	if len(names) < 1 {
		return
	}

	dns := names[0]
	if dns[len(dns)-1] == '.' {
		dns = dns[:len(dns)-1]
	}

	p.events <- programEventDns{
		key: key,
		dns: dns,
	}
}
