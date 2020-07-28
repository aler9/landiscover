package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"net"
	"os"
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

type programEvent interface {
	isProgramEvent()
}

type programEventArp struct {
	srcMac net.HardwareAddr
	srcIp  net.IP
}

func (programEventArp) isProgramEvent() {}

type programEventDns struct {
	key nodeKey
	dns string
}

func (programEventDns) isProgramEvent() {}

type programEventMdns struct {
	srcMac     net.HardwareAddr
	srcIp      net.IP
	domainName string
}

func (programEventMdns) isProgramEvent() {}

type programEventNbns struct {
	srcMac net.HardwareAddr
	srcIp  net.IP
	name   string
}

func (programEventNbns) isProgramEvent() {}

type programEventUiGetData struct {
	resNodes chan map[nodeKey]*node
	done     chan struct{}
}

func (programEventUiGetData) isProgramEvent() {}

type programEventTerminate struct{}

func (programEventTerminate) isProgramEvent() {}

type program struct {
	passiveMode bool
	intf        *net.Interface
	ownIp       net.IP
	ls          *listener
	ma          *methodArp
	mm          *methodMdns
	mn          *methodNbns
	ui          *ui

	events chan programEvent
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

	p := &program{
		passiveMode: *argPassiveMode,
		intf:        intf,
		ownIp:       ownIp,
		events:      make(chan programEvent),
	}

	err = newListener(p)
	if err != nil {
		return err
	}

	err = newMethodArp(p)
	if err != nil {
		return err
	}

	err = newMethodMdns(p)
	if err != nil {
		return err
	}

	err = newMethodNbns(p)
	if err != nil {
		return err
	}

	err = newUi(p)
	if err != nil {
		return err
	}

	p.run()
	return nil
}

func (p *program) run() {
	go p.ls.run()
	go p.ma.run()
	go p.mm.run()
	go p.mn.run()
	go p.ui.run()

	nodes := make(map[nodeKey]*node)

outer:
	for rawEvt := range p.events {
		switch evt := rawEvt.(type) {
		case programEventArp:
			key := newNodeKey(evt.srcMac, evt.srcIp)

			if _, ok := nodes[key]; !ok {
				nodes[key] = &node{
					lastSeen: time.Now(),
					mac:      evt.srcMac,
					ip:       evt.srcIp,
				}

				if p.passiveMode == false {
					go p.dnsRequest(key, evt.srcIp)
					go p.mm.request(evt.srcIp)
					go p.mn.request(evt.srcIp)
				}

				// update last seen
			} else {
				nodes[key].lastSeen = time.Now()
			}

		case programEventDns:
			nodes[evt.key].dns = evt.dns

		case programEventMdns:
			key := newNodeKey(evt.srcMac, evt.srcIp)

			if _, ok := nodes[key]; !ok {
				nodes[key] = &node{
					lastSeen: time.Now(),
					mac:      evt.srcMac,
					ip:       evt.srcIp,
				}
			}

			nodes[key].lastSeen = time.Now()
			if nodes[key].mdns != evt.domainName {
				nodes[key].mdns = evt.domainName
			}

		case programEventNbns:
			key := newNodeKey(evt.srcMac, evt.srcIp)

			if _, has := nodes[key]; !has {
				nodes[key] = &node{
					lastSeen: time.Now(),
					mac:      evt.srcMac,
					ip:       evt.srcIp,
				}
			}

			nodes[key].lastSeen = time.Now()
			if nodes[key].nbns != evt.name {
				nodes[key].nbns = evt.name
			}

		case programEventUiGetData:
			evt.resNodes <- nodes
			<-evt.done

		case programEventTerminate:
			break outer
		}
	}

	go func() {
		for rawEvt := range p.events {
			switch evt := rawEvt.(type) {
			case programEventUiGetData:
				evt.resNodes <- nil
			}
		}
	}()

	p.ui.close()
}

func main() {
	err := newProgram()
	if err != nil {
		fmt.Println("ERR:", err)
		os.Exit(1)
	}
}
