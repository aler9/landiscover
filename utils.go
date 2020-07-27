package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"strings"

	"github.com/google/gopacket/macs"
)

func defaultInterfaceName() (string, error) {
	intfs, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, in := range intfs {
		// must not be loopback
		if (in.Flags & net.FlagLoopback) != 0 {
			continue
		}

		// must be broadcast capable
		if (in.Flags & net.FlagBroadcast) == 0 {
			continue
		}

		addrs, err := in.Addrs()
		if err != nil {
			continue
		}

		// must have a valid ipv4
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok {
				if ip4 := ipn.IP.To4(); ip4 != nil {
					return in.Name, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no interfaces found")
}

func macVendor(mac net.HardwareAddr) string {
	var pref [3]byte
	copy(pref[:], mac[:3])
	if v, ok := macs.ValidMACPrefixMap[pref]; ok {
		return v
	}
	return "unknown"
}

func copyMac(in net.HardwareAddr) net.HardwareAddr {
	ret := net.HardwareAddr(make([]byte, 6))
	copy(ret, in)
	return ret
}

func copyIp(in net.IP) net.IP {
	ret := net.IP(make([]byte, 4))
	copy(ret, in)
	return ret
}

func randUint16() uint16 {
	return uint16(rand.Uint32())
}

func randAvailableIps(ownIp net.IP) []net.IP {
	var entries []net.IP
	for i := byte(1); i <= 254; i++ {
		eip := make([]byte, 4)
		copy(eip, ownIp)
		eip[3] = i
		if bytes.Equal(eip, ownIp) == true { // skip own ip
			continue
		}
		entries = append(entries, eip)
	}

	rand.Shuffle(len(entries), func(i, j int) {
		entries[i], entries[j] = entries[j], entries[i]
	})

	return entries
}

// <size>part<size>part
func dnsQueryDecode(data []byte, start int) (string, int) {
	var read []byte
	toread := uint8(0)
	pos := start

	for ; true; pos++ {
		if pos >= len(data) { // decoding terminated before null character
			return "", -1
		}
		if data[pos] == 0x00 {
			if toread > 0 { // decoding terminated before part parsing
				return "", -1
			}
			break // query correctly decoded
		}

		if toread == 0 { // we need a size or pointer
			if len(read) > 0 { // add separator
				read = append(read, '.')
			}

			if (data[pos] & 0xC0) == 0xC0 { // pointer
				ptr := int(binary.BigEndian.Uint16(data[pos:pos+2]) & 0x3FFF)
				pos++ // skip next byte

				substr, subread := dnsQueryDecode(data, ptr)
				if subread <= 0 {
					return "", -1
				}
				read = append(read, []byte(substr)...)
				break // query correctly decoded

			} else { // size
				toread = data[pos]
			}

		} else { // byte inside part
			read = append(read, data[pos])
			toread--
		}
	}
	return string(read), (pos + 1 - start)
}

func dnsQueryEncode(in string) []byte {
	var ret []byte
	for _, part := range strings.Split(in, ".") {
		bpart := []byte(part)
		ret = append(ret, uint8(len(bpart)))
		ret = append(ret, bpart...)
	}
	ret = append(ret, uint8(0))
	return ret
}
