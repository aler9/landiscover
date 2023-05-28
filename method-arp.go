package main

import (
	"bytes"
	"net"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

const (
	arpPeriod     = 50 * time.Millisecond
	arpScanPeriod = 10 * time.Second
)

type methodArp struct {
	p *program

	listen chan []byte
}

func newMethodArp(p *program) error {
	ma := &methodArp{
		p:      p,
		listen: make(chan []byte),
	}

	p.ma = ma
	return nil
}

func (ma *methodArp) run() {
	go ma.runListener()

	if !ma.p.passiveMode {
		go ma.runPeriodicRequests()
	}
}

func (ma *methodArp) runListener() {
	var decodedLayers []gopacket.LayerType
	var eth layers.Ethernet
	var arp layers.ARP
	var padding gopacket.Payload

	parser := gopacket.NewDecodingLayerParser(layers.LayerTypeEthernet,
		&eth,
		&arp,
		&padding)

	parse := func(raw []byte) {
		if err := parser.DecodeLayers(raw, &decodedLayers); err != nil {
			return
		}

		if arp.Protocol != layers.EthernetTypeIPv4 ||
			arp.HwAddressSize != 6 ||
			arp.ProtAddressSize != 4 {
			return
		}

		if bytes.Equal(arp.SourceProtAddress, []byte{0, 0, 0, 0}) {
			return
		}

		srcMac := copyMac(arp.SourceHwAddress)
		srcIP := copyIP(arp.SourceProtAddress)

		// ethernet mac and arp mac must correspond
		if !bytes.Equal(arp.SourceHwAddress, eth.SrcMAC) {
			return
		}

		ma.p.arp <- arpReq{
			srcMac: srcMac,
			srcIP:  srcIP,
		}
	}

	for raw := range ma.listen {
		parse(raw)
		ma.p.ls.listenDone <- struct{}{}
	}
}

func (ma *methodArp) runPeriodicRequests() {
	eth := layers.Ethernet{
		SrcMAC:       ma.p.intf.HardwareAddr,
		DstMAC:       net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		EthernetType: layers.EthernetTypeARP,
	}
	arp := layers.ARP{
		AddrType:          layers.LinkTypeEthernet,
		Protocol:          layers.EthernetTypeIPv4,
		HwAddressSize:     6,
		ProtAddressSize:   4,
		Operation:         layers.ARPRequest,
		SourceHwAddress:   ma.p.intf.HardwareAddr,
		SourceProtAddress: ma.p.ownIP,
		DstHwAddress:      []byte{0, 0, 0, 0, 0, 0},
	}

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}

	for {
		for _, dstAddr := range randAvailableIPs(ma.p.ownIP) {
			arp.DstProtAddress = dstAddr
			if err := gopacket.SerializeLayers(buf, opts, &eth, &arp); err != nil {
				panic(err)
			}

			err := ma.p.ls.socket.Write(buf.Bytes())
			if err != nil {
				panic(err)
			}

			// more results if there's a minimum delay between arps
			time.Sleep(arpPeriod)
		}
		time.Sleep(arpScanPeriod)
	}
}
