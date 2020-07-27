package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	"gopkg.in/alecthomas/kingpin.v2"
)

var Version = "v0.0.0"

type nodeKey struct {
	mac [6]byte
	ip  [4]byte
}

func newNodeKey(mac []byte, ip []byte) nodeKey {
	key := nodeKey{}
	copy(key.mac[:], mac[:])
	copy(key.ip[:], ip)
	return key
}

type node struct {
	lastSeen time.Time
	mac      net.HardwareAddr
	ip       net.IP
	dns      string
	nbns     string
	mdns     string
}

type program struct {
	passiveMode bool
	intf        *net.Interface
	ownIp       net.IP
	socket      *rawSocket

	mutex        sync.Mutex
	nodes        map[nodeKey]*node
	listenDone   chan struct{}
	listenArp    chan []byte
	listenNbns   chan []byte
	listenMdns   chan []byte
	uiDrawQueued bool
}

func newProgram() error {
	k := kingpin.New("landiscover",
		"landiscover "+Version+"\n\nMachine and service discovery tool.")

	argInterface := k.Arg("interface", "Interface to listen to").String()
	argPassiveMode := k.Flag("passive", "do not send any packet").Default("false").Bool()

	kingpin.MustParse(k.Parse(os.Args[1:]))

	if os.Getuid() != 0 {
		return fmt.Errorf("you must be root")
	}

	rand.Seed(time.Now().UnixNano())
	layerNbnsInit()
	layerMdnsInit()

	intfName, err := func() (string, error) {
		if len(*argInterface) > 1 {
			return *argInterface, nil
		}

		return defaultInterfaceName()
	}()
	if err != nil {
		return err
	}

	intf, err := func() (*net.Interface, error) {
		intf, err := net.InterfaceByName(intfName)
		if err != nil {
			return nil, fmt.Errorf("invalid interface: %s", intfName)
		}

		if (intf.Flags & net.FlagBroadcast) == 0 {
			return nil, fmt.Errorf("interface does not support broadcast")
		}

		return intf, nil
	}()
	if err != nil {
		return err
	}

	ownIp, err := func() (net.IP, error) {
		addrs, err := intf.Addrs()
		if err != nil {
			return nil, err
		}

		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok {
				if ip4 := ipn.IP.To4(); ip4 != nil {
					if bytes.Equal(ipn.Mask, []byte{255, 255, 255, 0}) {
						return ip4, nil
					}
				}
			}
		}

		return nil, fmt.Errorf("no valid ip found")
	}()
	if err != nil {
		return err
	}

	socket, err := newRawSocket(intf)
	if err != nil {
		return err
	}

	p := &program{
		passiveMode:  *argPassiveMode,
		intf:         intf,
		ownIp:        ownIp,
		socket:       socket,
		nodes:        make(map[nodeKey]*node),
		listenDone:   make(chan struct{}),
		listenArp:    make(chan []byte),
		listenNbns:   make(chan []byte),
		listenMdns:   make(chan []byte),
		uiDrawQueued: true,
	}

	p.arpInit()
	p.nbnsInit()
	p.mdnsInit()

	go p.listen()

	p.ui()
	return nil
}

func (p *program) listen() {
	for {
		raw, err := p.socket.Read()
		if err != nil {
			panic(err)
		}

		p.listenArp <- raw
		p.listenNbns <- raw
		p.listenMdns <- raw

		// join before refilling buffer
		for i := 0; i < 3; i++ {
			<-p.listenDone
		}
	}
}

func main() {
	err := newProgram()
	if err != nil {
		fmt.Println("ERR:", err)
		os.Exit(1)
	}
}
