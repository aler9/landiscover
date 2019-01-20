package main

import (
    "fmt"
    "net"
    "time"
    "bytes"
    "strings"
    "github.com/google/gopacket"
    "github.com/google/gopacket/layers"
)

func (ls *LanDiscover) mdnsInit() {
    go ls.mdnsListen()

    if *argPassiveMode == false {
        // continuously poll mdns to detect changes or skipped hosts
        go ls.mdnsPeriodicRequests()
    }
}

func (ls *LanDiscover) mdnsListen() {
    var decodedLayers []gopacket.LayerType
    var eth layers.Ethernet
    var ip layers.IPv4
    var udp layers.UDP
    var mdns LayerMdns

    parser := gopacket.NewDecodingLayerParser(layers.LayerTypeEthernet,
        &eth,
        &ip,
        &udp,
        &mdns)

    parse := func(raw []byte) {
        if err := parser.DecodeLayers(raw, &decodedLayers); err != nil {
            return
        }

        if len(mdns.Answers) == 0 {
            return
        }

        srcMac := copyMac(eth.SrcMAC)
        srcIp := copyIp(ip.SrcIP)

        domainName := func() string {
            for _,a := range mdns.Answers {
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
            }

            ls.nodes[key].LastSeen = time.Now()
            if ls.nodes[key].Mdns != domainName {
                ls.nodes[key].Mdns = domainName
            }
            ls.uiQueueDraw()
        }()
    }

    for raw := range ls.listenMdns {
        parse(raw)
        ls.listenDone <- struct{}{}
    }
}

func (ls *LanDiscover) mdnsRequest(destIp net.IP) {
    mac,_ := net.ParseMAC("01:00:5e:00:00:fb")
    eth := layers.Ethernet{
		SrcMAC: ls.intf.HardwareAddr,
		DstMAC: mac,
		EthernetType: layers.EthernetTypeIPv4,
	}
    ip := layers.IPv4{
        Version: 4,
        TTL: 255,
        Id: randUint16(),
        Protocol: layers.IPProtocolUDP,
        SrcIP: ls.myIp,
        DstIP: net.ParseIP("224.0.0.251"), // TODO: provare unicast
    }
    udp := layers.UDP{
        SrcPort: MDNS_PORT,
        DstPort: MDNS_PORT,
    }
    udp.SetNetworkLayerForChecksum(&ip)
    mdns := LayerMdns{
        TransactionId: 0,
        Questions: []MdnsQuestion{
            MdnsQuestion{
                Query: fmt.Sprintf("%d.%d.%d.%d.in-addr.arpa", destIp[3], destIp[2], destIp[1], destIp[0]),
                Type: 0x0C, // domain pointer
                Class: 1, // IN
            },
        },
    }

    buf := gopacket.NewSerializeBuffer()
    opts := gopacket.SerializeOptions{
        FixLengths: true,
        ComputeChecksums: true,
    }
    if err := gopacket.SerializeLayers(buf, opts, &eth, &ip, &udp, &mdns); err != nil {
        panic(err)
    }

    if err := ls.handle.WritePacketData(buf.Bytes()); err != nil {
        panic(err)
    }
}

func (ls *LanDiscover) mdnsPeriodicRequests() {
    for {
        for _,dstAddr := range randAvailableIps(ls.myIp) {
            ls.mdnsRequest(dstAddr)
            time.Sleep(MDNS_PERIOD) // about 1 minute for a full scan
        }
    }
}
