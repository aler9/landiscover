package main

import (
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

type methodMdns struct {
	p *program

	listen chan []byte
}

func newMethodMdns(p *program) error {
	mm := &methodMdns{
		p:      p,
		listen: make(chan []byte),
	}

	p.mm = mm
	return nil
}

func (mm *methodMdns) run() {
	go mm.runListener()

	if !mm.p.passiveMode {
		// continuously poll mdns in order to detect changes or skipped hosts
		go mm.runPeriodicRequests()
	}
}

func (mm *methodMdns) runListener() {
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

		if udp.DstPort != mdnsPort && udp.SrcPort != mdnsPort {
			return
		}

		if len(mdns.Answers) == 0 {
			return
		}

		srcMac := copyMac(eth.SrcMAC)
		srcIP := copyIP(ip.SrcIP)

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
				mdnsIP := net.ParseIP(fmt.Sprintf("%s.%s.%s.%s", m[4], m[3], m[2], m[1])).To4()
				if !mdnsIP.Equal(srcIP) {
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

		mm.p.mdns <- mdnsReq{
			srcMac:     srcMac,
			srcIP:      srcIP,
			domainName: domainName,
		}
	}

	for raw := range mm.listen {
		parse(raw)
		mm.p.ls.listenDone <- struct{}{}
	}
}

func (mm *methodMdns) request(destIP net.IP) {
	mac, _ := net.ParseMAC("01:00:5e:00:00:fb")
	eth := layers.Ethernet{
		SrcMAC:       mm.p.intf.HardwareAddr,
		DstMAC:       mac,
		EthernetType: layers.EthernetTypeIPv4,
	}

	v, err := randUint16()
	if err != nil {
		panic(err)
	}

	ip := layers.IPv4{
		Version:  4,
		TTL:      255,
		Id:       v,
		Protocol: layers.IPProtocolUDP,
		SrcIP:    mm.p.ownIP,
		DstIP:    net.ParseIP("224.0.0.251"), // TODO: provare unicast
	}
	udp := layers.UDP{
		SrcPort: mdnsPort,
		DstPort: mdnsPort,
	}

	err = udp.SetNetworkLayerForChecksum(&ip)
	if err != nil {
		panic(err)
	}

	mdns := layerMdns{
		TransactionID: 0,
		Questions: []mdnsQuestion{
			{
				Query: fmt.Sprintf("%d.%d.%d.%d.in-addr.arpa", destIP[3], destIP[2], destIP[1], destIP[0]),
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

	err = gopacket.SerializeLayers(buf, opts, &eth, &ip, &udp, &mdns)
	if err != nil {
		panic(err)
	}

	err = mm.p.ls.socket.Write(buf.Bytes())
	if err != nil {
		panic(err)
	}
}

func (mm *methodMdns) runPeriodicRequests() {
	for {
		ips, err := randAvailableIPs(mm.p.ownIP)
		if err != nil {
			panic(err)
		}

		for _, dstAddr := range ips {
			mm.request(dstAddr)
			time.Sleep(mdnsPeriod) // about 1 minute for a full scan
		}
	}
}
