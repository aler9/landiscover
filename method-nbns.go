package main

import (
	"net"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

const (
	nbnsPeriod = 200 * time.Millisecond
)

func (p *program) nbnsInit() {
	go p.nbnsListen()
}

func (p *program) nbnsListen() {
	var decodedLayers []gopacket.LayerType
	var eth layers.Ethernet
	var ip layers.IPv4
	var udp layers.UDP
	var nbns layerNbns

	parser := gopacket.NewDecodingLayerParser(layers.LayerTypeEthernet,
		&eth,
		&ip,
		&udp,
		&nbns)

	parse := func(raw []byte) {
		if err := parser.DecodeLayers(raw, &decodedLayers); err != nil {
			return
		}

		if udp.DstPort != 137 {
			return
		}

		if len(nbns.Answers) != 1 {
			return
		}

		name := func() string {
			for _, n := range nbns.Answers[0].Names {
				if n.Type == 0x20 { // service name
					return n.Name
				}
			}
			return ""
		}()
		if name == "" {
			return
		}

		srcMac := copyMac(eth.SrcMAC)
		srcIp := copyIp(ip.SrcIP)

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
			if p.nodes[key].nbns != name {
				p.nodes[key].nbns = name
			}
			p.uiQueueDraw()
		}()
	}

	for raw := range p.listenNbns {
		parse(raw)
		p.listenDone <- struct{}{}
	}
}

func (p *program) nbnsRequest(destIp net.IP) {
	localAddr := &net.UDPAddr{}
	remoteAddr := &net.UDPAddr{
		IP:   destIp,
		Port: nbnsPort,
	}
	conn, err := net.DialUDP("udp", localAddr, remoteAddr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	nbns := layerNbns{
		TransactionId: randUint16(),
		Questions: []NbnsQuestion{
			{
				Query: "CKAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
				Type:  0x21, // NB_STAT
				Class: 1,    // IN
			},
		},
	}

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	if err := gopacket.SerializeLayers(buf, opts, &nbns); err != nil {
		panic(err)
	}

	if _, err := conn.Write(buf.Bytes()); err != nil {
		panic(err)
	}

	// close immediately the connection even if this generates a "ICMP"
	// "destination unreachable". Otherwise connection count would increment with time
}

func (p *program) nbnsPeriodicRequests() {
	for {
		for _, dstAddr := range randAvailableIps(p.ownIp) {
			p.nbnsRequest(dstAddr)
			time.Sleep(nbnsPeriod) // about 1 minute for a full scan
		}
	}
}
