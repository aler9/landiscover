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

var version = "v0.0.0"

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

type arpReq struct {
	srcMac net.HardwareAddr
	srcIP  net.IP
}

type dnsReq struct {
	key nodeKey
	dns string
}

type mdnsReq struct {
	srcMac     net.HardwareAddr
	srcIP      net.IP
	domainName string
}

type nbnsReq struct {
	srcMac net.HardwareAddr
	srcIP  net.IP
	name   string
}

type uiGetDataReq struct {
	resNodes chan map[nodeKey]*node
	done     chan struct{}
}

type program struct {
	passiveMode bool
	intf        *net.Interface
	ownIP       net.IP
	ls          *listener
	ma          *methodArp
	mm          *methodMdns
	mn          *methodNbns
	ui          *ui

	arp       chan arpReq
	dns       chan dnsReq
	mdns      chan mdnsReq
	nbns      chan nbnsReq
	uiGetData chan uiGetDataReq
	terminate chan struct{}
}

func newProgram() error {
	k := kingpin.New("landiscover",
		"landiscover "+version+"\n\nMachine and service discovery tool.")

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

	ownIP, err := func() (net.IP, error) {
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
		ownIP:       ownIP,
		arp:         make(chan arpReq),
		dns:         make(chan dnsReq),
		mdns:        make(chan mdnsReq),
		nbns:        make(chan nbnsReq),
		uiGetData:   make(chan uiGetDataReq),
		terminate:   make(chan struct{}),
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

	err = newUI(p)
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
	for {
		select {
		case req := <-p.arp:
			key := newNodeKey(req.srcMac, req.srcIP)

			if _, ok := nodes[key]; !ok {
				nodes[key] = &node{
					lastSeen: time.Now(),
					mac:      req.srcMac,
					ip:       req.srcIP,
				}

				if !p.passiveMode {
					go p.dnsRequest(key, req.srcIP)
					go p.mm.request(req.srcIP)
					go p.mn.request(req.srcIP)
				}

				// update last seen
			} else {
				nodes[key].lastSeen = time.Now()
			}

		case req := <-p.dns:
			nodes[req.key].dns = req.dns

		case req := <-p.mdns:
			key := newNodeKey(req.srcMac, req.srcIP)

			if _, ok := nodes[key]; !ok {
				nodes[key] = &node{
					lastSeen: time.Now(),
					mac:      req.srcMac,
					ip:       req.srcIP,
				}
			}

			nodes[key].lastSeen = time.Now()
			if nodes[key].mdns != req.domainName {
				nodes[key].mdns = req.domainName
			}

		case req := <-p.nbns:
			key := newNodeKey(req.srcMac, req.srcIP)

			if _, has := nodes[key]; !has {
				nodes[key] = &node{
					lastSeen: time.Now(),
					mac:      req.srcMac,
					ip:       req.srcIP,
				}
			}

			nodes[key].lastSeen = time.Now()
			if nodes[key].nbns != req.name {
				nodes[key].nbns = req.name
			}

		case req := <-p.uiGetData:
			req.resNodes <- nodes
			<-req.done

		case <-p.terminate:
			break outer
		}
	}

	go func() {
		for {
			select {
			case _, ok := <-p.arp:
				if !ok {
					return
				}
			case <-p.dns:
			case <-p.mdns:
			case <-p.nbns:
			case req := <-p.uiGetData:
				req.resNodes <- nil
			}
		}
	}()

	p.ui.close()

	/*close(p.arp)
	close(p.dns)
	close(p.mdns)
	close(p.nbns)
	close(p.uiGetData)*/
}

func main() {
	err := newProgram()
	if err != nil {
		fmt.Println("ERR:", err)
		os.Exit(1)
	}
}
