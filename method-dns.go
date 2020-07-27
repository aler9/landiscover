package main

import (
	"net"
)

func (p *program) doDnsRequest(key nodeKey, destIp net.IP) {
	names, err := net.LookupAddr(destIp.String())
	if err != nil {
		return
	}

	if len(names) < 1 {
		return
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()
	name := names[0]
	if name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}
	p.nodes[key].dns = name
	p.uiQueueDraw()
}
