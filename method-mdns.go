package main

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

const (
	mdnsPeriod = 200 * time.Millisecond
)

func (p *program) mdnsInit() {
	go p.mdnsListen()

	if p.passiveMode == false {
		// continuously poll mdns to detect changes or skipped hosts
		go p.mdnsPeriodicRequests()
	}
}

func (p *program) mdnsListen() {
	var decodedLayers []gopacket.LayerType
	var eth layers.Ethernet
	var ip layers.IPv4
	var udp layers.UDP
	var mdns layerMdns

	parser := gopacket.NewDecodingLayerParser(layers.LayerTypeEthernet,
		&eth,
		&ip,
		&udp,
		&mdns)

	parse := func(raw []byte) {
		if err := parser.DecodeLayers(raw, &decodedLayers); err != nil {
			return
		}

		if udp.DstPort != mdnsPort {
			return
		}

		if len(mdns.Answers) == 0 {
			return
		}

		srcMac := copyMac(eth.SrcMAC)
		srcIp := copyIp(ip.SrcIP)

		domainName := func() string {
			for _, a := range mdns.Answers {
				domainName := a.DomainName
				if a.Type != 12 { // PTR
					continue
				}

				m := reMdnsQueryLocal.FindStringSubmatch(a.Query)
				if m == nil {
					continue
				}

				// accept only if mdns ip matches with sender ip
				mdnsIp := net.ParseIP(fmt.Sprintf("%s.%s.%s.%s", m[4], m[3], m[2], m[1])).To4()
				if bytes.Equal(mdnsIp, srcIp) == false {
					continue
				}

				return domainName
			}
			return ""
		}()
		if domainName == "" {
			return
		}

		domainName = strings.TrimSuffix(domainName, ".local")
		key := newNodeKey(srcMac, srcIp)

		func() {
			p.mutex.Lock()
			defer p.mutex.Unlock()

			if _, has := p.nodes[key]; !has {
				p.nodes[key] = &node{
					lastSeen: time.Now(),
					mac:      srcMac,
					ip:       srcIp,
				}
				p.uiQueueDraw()
			}

			p.nodes[key].lastSeen = time.Now()
			if p.nodes[key].mdns != domainName {
				p.nodes[key].mdns = domainName
			}
			p.uiQueueDraw()
		}()
	}

	for raw := range p.listenMdns {
		parse(raw)
		p.listenDone <- struct{}{}
	}
}

func (p *program) mdnsRequest(destIp net.IP) {
	mac, _ := net.ParseMAC("01:00:5e:00:00:fb")
	eth := layers.Ethernet{
		SrcMAC:       p.intf.HardwareAddr,
		DstMAC:       mac,
		EthernetType: layers.EthernetTypeIPv4,
	}
	ip := layers.IPv4{
		Version:  4,
		TTL:      255,
		Id:       randUint16(),
		Protocol: layers.IPProtocolUDP,
		SrcIP:    p.ownIp,
		DstIP:    net.ParseIP("224.0.0.251"), // TODO: provare unicast
	}
	udp := layers.UDP{
		SrcPort: mdnsPort,
		DstPort: mdnsPort,
	}
	udp.SetNetworkLayerForChecksum(&ip)
	mdns := layerMdns{
		TransactionId: 0,
		Questions: []MdnsQuestion{
			{
				Query: fmt.Sprintf("%d.%d.%d.%d.in-addr.arpa", destIp[3], destIp[2], destIp[1], destIp[0]),
				Type:  0x0C, // domain pointer
				Class: 1,    // IN
			},
		},
	}

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	if err := gopacket.SerializeLayers(buf, opts, &eth, &ip, &udp, &mdns); err != nil {
		panic(err)
	}

	err := p.socket.Write(buf.Bytes())
	if err != nil {
		panic(err)
	}
}

func (p *program) mdnsPeriodicRequests() {
	for {
		for _, dstAddr := range randAvailableIps(p.ownIp) {
			p.mdnsRequest(dstAddr)
			time.Sleep(mdnsPeriod) // about 1 minute for a full scan
		}
	}
}
