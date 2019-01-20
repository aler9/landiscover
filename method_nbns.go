package main

import (
    "net"
    "time"
    "github.com/google/gopacket"
    "github.com/google/gopacket/layers"
)

func (ls *LanDiscover) nbnsInit() {
    go ls.nbnsListen()
}

func (ls *LanDiscover) nbnsListen() {
    var decodedLayers []gopacket.LayerType
    var eth layers.Ethernet
    var ip layers.IPv4
    var udp layers.UDP
    var nbns LayerNbns

    parser := gopacket.NewDecodingLayerParser(layers.LayerTypeEthernet,
        &eth,
        &ip,
        &udp,
        &nbns)

    parse := func(raw []byte) {
        if err := parser.DecodeLayers(raw, &decodedLayers); err != nil {
            return
        }
        if len(nbns.Answers) != 1 {
            return
        }

        name := func() string {
            for _,n := range nbns.Answers[0].Names {
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
            if ls.nodes[key].Nbns != name {
                ls.nodes[key].Nbns = name
            }
            ls.uiQueueDraw()
        }()
    }

    for raw := range ls.listenNbns {
        parse(raw)
        ls.listenDone <- struct{}{}
    }
}

func (ls *LanDiscover) nbnsRequest(destIp net.IP) {
    localAddr := &net.UDPAddr{}
    remoteAddr := &net.UDPAddr{
        IP: destIp,
        Port: NBNS_PORT,
    }
	conn,err := net.DialUDP("udp", localAddr, remoteAddr)
	if err != nil {
        panic(err)
	}
    defer conn.Close()

    nbns := LayerNbns{
        TransactionId: randUint16(),
        Questions: []NbnsQuestion{
            NbnsQuestion{
                Query: "CKAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
                Type: 0x21, // NB_STAT
                Class: 1, // IN
            },
        },
    }

    buf := gopacket.NewSerializeBuffer()
    opts := gopacket.SerializeOptions{
        FixLengths: true,
        ComputeChecksums: true,
    }
    if err := gopacket.SerializeLayers(buf, opts, &nbns); err != nil {
        panic(err)
    }

    if _,err := conn.Write(buf.Bytes()); err != nil {
        panic(err)
    }

    // close immediately the connection even if this generates a "ICMP"
    // "destination unreachable". Otherwise connection count would increment with time
}

func (ls *LanDiscover) nbnsPeriodicRequests() {
    for {
        for _,dstAddr := range randAvailableIps(ls.myIp) {
            ls.nbnsRequest(dstAddr)
            time.Sleep(NBNS_PERIOD) // about 1 minute for a full scan
        }
    }
}
