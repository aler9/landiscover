package main

import (
	"net"
)

func (ls *LanDiscover) doDnsRequest(key NodeKey, destIp net.IP) {
	names, err := net.LookupAddr(destIp.String())
	if err != nil {
		return
	}

	if len(names) < 1 {
		return
	}

	ls.mutex.Lock()
	defer ls.mutex.Unlock()
	name := names[0]
	if name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}
	ls.nodes[key].Dns = name
	ls.uiQueueDraw()
}
