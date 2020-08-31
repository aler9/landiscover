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

type methodNbns struct {
	p *program

	listen chan []byte
}

func newMethodNbns(p *program) error {
	mn := &methodNbns{
		p:      p,
		listen: make(chan []byte),
	}

	p.mn = mn
	return nil
}

func (mn *methodNbns) run() {
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

		if udp.DstPort != nbnsPort && udp.SrcPort != nbnsPort {
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

		mn.p.nbns <- nbnsReq{
			srcMac: srcMac,
			srcIp:  srcIp,
			name:   name,
		}
	}

	for raw := range mn.listen {
		parse(raw)
		mn.p.ls.listenDone <- struct{}{}
	}
}

func (mn *methodNbns) request(destIp net.IP) {
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
		Questions: []nbnsQuestion{
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
