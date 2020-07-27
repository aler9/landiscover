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

func (p *program) arpInit() {
	go p.arpListen()
	if p.passiveMode == false {
		go p.arpPeriodicRequests()
	}
}

func (p *program) arpListen() {
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

		if bytes.Equal(arp.SourceProtAddress, []byte{0, 0, 0, 0}) == true {
			return
		}

		srcMac := copyMac(arp.SourceHwAddress)
		srcIp := copyIp(arp.SourceProtAddress)

		// ethernet mac and arp mac must correspond
		if bytes.Equal(arp.SourceHwAddress, eth.SrcMAC) == false {
			return
		}

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

				if p.passiveMode == false {
					go p.doDnsRequest(key, srcIp)
					go p.nbnsRequest(srcIp)
					go p.mdnsRequest(srcIp)
				}

				// update last seen
			} else {
				p.nodes[key].lastSeen = time.Now()
				p.uiQueueDraw()
			}
		}()
	}

	for raw := range p.listenArp {
		parse(raw)
		p.listenDone <- struct{}{}
	}
}

func (p *program) arpPeriodicRequests() {
	eth := layers.Ethernet{
		SrcMAC:       p.intf.HardwareAddr,
		DstMAC:       net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		EthernetType: layers.EthernetTypeARP,
	}
	arp := layers.ARP{
		AddrType:          layers.LinkTypeEthernet,
		Protocol:          layers.EthernetTypeIPv4,
		HwAddressSize:     6,
		ProtAddressSize:   4,
		Operation:         layers.ARPRequest,
		SourceHwAddress:   p.intf.HardwareAddr,
		SourceProtAddress: p.ownIp,
		DstHwAddress:      []byte{0, 0, 0, 0, 0, 0},
	}

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}

	for {
		for _, dstAddr := range randAvailableIps(p.ownIp) {
			arp.DstProtAddress = dstAddr
			if err := gopacket.SerializeLayers(buf, opts, &eth, &arp); err != nil {
				panic(err)
			}

			err := p.socket.Write(buf.Bytes())
			if err != nil {
				panic(err)
			}

			// more results if there's a minimum delay between arps
			time.Sleep(arpPeriod)
		}
		time.Sleep(arpScanPeriod)
	}
}
