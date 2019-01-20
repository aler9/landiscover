package main

import (
    "net"
    "time"
    "bytes"
    "github.com/google/gopacket"
    "github.com/google/gopacket/layers"
)

func (ls *LanDiscover) arpInit() {
    go ls.arpListen()
    if *argPassiveMode == false {
        go ls.arpPeriodicRequests()
    }
}

func (ls *LanDiscover) arpListen() {
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

        if bytes.Equal(arp.SourceProtAddress, []byte{ 0, 0, 0, 0 }) == true {
            return
        }

        srcMac := copyMac(arp.SourceHwAddress)
        srcIp := copyIp(arp.SourceProtAddress)

        // ethernet mac and arp mac must correspond
        if bytes.Equal(arp.SourceHwAddress, eth.SrcMAC) == false {
            return
        }

        key := FillNodeKey(srcMac, srcIp)

        func() {
            ls.mutex.Lock()
            defer ls.mutex.Unlock()

            if _,has := ls.nodes[key]; !has {
                ls.nodes[key] = &Node{
                    LastSeen: time.Now(),
                    Mac: srcMac,
                    Ip: srcIp,
                }
                ls.uiQueueDraw()

                if *argPassiveMode == false {
                    go ls.doDnsRequest(key, srcIp)
                    go ls.nbnsRequest(srcIp)
                    go ls.mdnsRequest(srcIp)
                }

            // update last seen
            } else {
                ls.nodes[key].LastSeen = time.Now()
                ls.uiQueueDraw()
            }
        }()
    }

    for raw := range ls.listenArp {
        parse(raw)
        ls.listenDone <- struct{}{}
    }
}

func (ls *LanDiscover) arpPeriodicRequests() {
	eth := layers.Ethernet{
		SrcMAC: ls.intf.HardwareAddr,
		DstMAC: net.HardwareAddr{ 0xff, 0xff, 0xff, 0xff, 0xff, 0xff },
		EthernetType: layers.EthernetTypeARP,
	}
	arp := layers.ARP{
		AddrType: layers.LinkTypeEthernet,
		Protocol: layers.EthernetTypeIPv4,
		HwAddressSize: 6,
		ProtAddressSize: 4,
		Operation: layers.ARPRequest,
		SourceHwAddress: ls.intf.HardwareAddr,
		SourceProtAddress: ls.myIp,
		DstHwAddress: []byte{ 0, 0, 0, 0, 0, 0 },
    }

    buf := gopacket.NewSerializeBuffer()
    opts := gopacket.SerializeOptions{
        FixLengths: true,
        ComputeChecksums: true,
    }

    for {
        for _,dstAddr := range randAvailableIps(ls.myIp) {
    		arp.DstProtAddress = dstAddr
            if err := gopacket.SerializeLayers(buf, opts, &eth, &arp); err != nil {
                panic(err)
            }

            if err := ls.handle.WritePacketData(buf.Bytes()); err != nil {
                panic(err)
            }

            // more results if there's a minimum delay between arps
            time.Sleep(ARP_PERIOD)
        }
        time.Sleep(ARP_SCAN_PERIOD)
    }
}
