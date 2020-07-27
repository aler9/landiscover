package main

import (
    "os"
    "fmt"
    "net"
    "sync"
    "time"
    "bytes"
    "math/rand"

    "gopkg.in/alecthomas/kingpin.v2"
)

const (
    DRAW_PERIOD = 1 * time.Second
    ARP_PERIOD = 50 * time.Millisecond
    ARP_SCAN_PERIOD = 10 * time.Second
    MDNS_PERIOD = 200 * time.Millisecond
    NBNS_PERIOD = 200 * time.Millisecond
)

var (
  argInterface = kingpin.Arg("interface", "Interface to listen to").String()
  argPassiveMode = kingpin.Flag("passive", "Passive mode, listen without sending any packet").Default("false").Bool()
)

type NodeKey struct {
    Mac         [6]byte
    Ip          [4]byte
}

func FillNodeKey(mac []byte, ip []byte) NodeKey {
    key := NodeKey{}
    copy(key.Mac[:], mac[:])
    copy(key.Ip[:], ip)
    return key
}

type Node struct {
    LastSeen    time.Time
    Mac         net.HardwareAddr
    Ip          net.IP
    Dns         string
    Nbns        string
    Mdns        string
}

type LanDiscover struct {
    mutex               sync.Mutex
    nodes               map[NodeKey]*Node
    intf                *net.Interface
    myIp                net.IP
    socket *rawSocket
    listenDone          chan struct{}
    listenArp           chan []byte
    listenNbns          chan []byte
    listenMdns          chan []byte
    uiDrawQueued        bool
}

func main() {
    if os.Getuid() != 0 {
        panic(fmt.Errorf("you must be root."))
    }

    kingpin.Parse()

    rand.Seed(time.Now().UnixNano())
    LayerNbnsInit()
    LayerMdnsInit()

    ls := &LanDiscover{
        nodes: make(map[NodeKey]*Node),
        listenDone: make(chan struct{}),
        listenArp: make(chan []byte),
        listenNbns: make(chan []byte),
        listenMdns: make(chan []byte),
        uiDrawQueued: true,
    }

    interfaceName := func() string {
        if len(*argInterface) > 1 {
            return *argInterface
        }
        return ls.defaultInterface()
    }()
    ls.initInterface(interfaceName)

    ls.arpInit()
    ls.nbnsInit()
    ls.mdnsInit()

    go ls.listen()
    ls.ui()
}

func (ls *LanDiscover) defaultInterface() string {
    intfs,err := net.Interfaces()
    if err != nil {
        return ""
    }
    for _,in := range intfs {
        // must not be loopback
        if (in.Flags & net.FlagLoopback) != 0 {
            continue
        }

        // must be broadcast capable
        if (in.Flags & net.FlagBroadcast) == 0 {
            continue
        }

        // must have a valid ipv4
        addrs,err := in.Addrs()
        if err != nil {
            continue
        }
        for _,a := range addrs {
            if ipn,ok := a.(*net.IPNet); ok {
                if ip4 := ipn.IP.To4(); ip4 != nil {
                    return in.Name
                }
            }
        }
    }
    return ""
}

func (ls *LanDiscover) initInterface(intName string) {
    var err error
    ls.intf,err = net.InterfaceByName(intName)
    if err != nil {
        panic(fmt.Errorf("invalid interface: %s", intName))
    }

    if (ls.intf.Flags & net.FlagBroadcast) == 0 {
        panic("interface does not support broadcast")
    }

    addrs,err := ls.intf.Addrs()
    if err != nil {
        panic(err)
    }

    for _,a := range addrs {
        if ipn,ok := a.(*net.IPNet); ok {
            if ip4 := ipn.IP.To4(); ip4 != nil {
                if bytes.Equal(ipn.Mask, []byte{ 255, 255, 255, 0 }) {
                    ls.myIp = ip4
                    break
                }
            }
        }
    }
    if len(ls.myIp) == 0 {
        panic("no valid address found")
    }

    ls.socket, err = newRawSocket(ls.intf)
    if err != nil {
        panic(err)
    }
}

func (ls *LanDiscover) listen() {
    for {
        raw, err := ls.socket.Read()
        if err != nil {
            panic(err)
        }

        ls.listenArp <- raw
        ls.listenNbns <- raw
        ls.listenMdns <- raw

        // join before refilling buffer
        for i := 0; i < 3; i++ {
            <- ls.listenDone
        }
    }
}
