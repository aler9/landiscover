package main

import (
	"encoding/binary"
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"regexp"
)

const MDNS_PORT = 5353

var reMdnsQueryLocal = regexp.MustCompile("^([0-9]{1,3})\\.([0-9]{1,3})\\.([0-9]{1,3})\\.([0-9]{1,3})\\.in-addr\\.arpa$")

type LayerMdns struct {
	layers.BaseLayer
	TransactionId   uint16
	IsResponse      bool
	Opcode          uint8
	Questions       []MdnsQuestion
	Answers         []MdnsAnswer
	AuthorityCount  uint16
	AdditionalCount uint16
}

type MdnsQuestion struct {
	Query string
	Type  uint16
	Class uint16
}

type MdnsAnswer struct {
	Query      string
	Type       uint16
	Class      uint16
	TTL        uint32
	DomainName string
}

var LayerTypeMdns gopacket.LayerType

func LayerMdnsInit() {
	LayerTypeMdns = gopacket.RegisterLayerType(
		2501,
		gopacket.LayerTypeMetadata{
			Name:    "Mdns",
			Decoder: gopacket.DecodeFunc(decodeLayerMdns),
		},
	)
	layers.RegisterUDPPortLayerType(MDNS_PORT, LayerTypeMdns)
}

func decodeLayerMdns(data []byte, p gopacket.PacketBuilder) error {
	l := &LayerMdns{}
	err := l.DecodeFromBytes(data, p)
	if err != nil {
		return err
	}
	p.AddLayer(l)
	p.SetApplicationLayer(l)
	return nil
}

func (l *LayerMdns) LayerType() gopacket.LayerType {
	return LayerTypeMdns
}

func (l *LayerMdns) CanDecode() gopacket.LayerClass {
	return LayerTypeMdns
}

func (l *LayerMdns) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypeZero
}

func (l *LayerMdns) Payload() []byte {
	return nil
}

func (l *LayerMdns) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	l.BaseLayer = layers.BaseLayer{Contents: data[:]}

	l.TransactionId = binary.BigEndian.Uint16(data[0:2])
	l.IsResponse = (data[3] >> 7) == 0x01
	l.Opcode = uint8((data[3] >> 3) & 0x0F)
	questionCount := binary.BigEndian.Uint16(data[4:6])
	answerCount := binary.BigEndian.Uint16(data[6:8])
	l.AuthorityCount = binary.BigEndian.Uint16(data[8:10])
	l.AdditionalCount = binary.BigEndian.Uint16(data[10:12])
	pos := 12

	if questionCount > 0 {
		return fmt.Errorf("is question, unsupported")
	}

	l.Answers = nil
	for i := uint16(0); i < answerCount; i++ {
		a := MdnsAnswer{}

		var read int
		a.Query, read = dnsQueryDecode(data, pos)
		if read <= 0 {
			return fmt.Errorf("answer query: invalid string (%v)", data)
		}
		pos += read

		a.Type = binary.BigEndian.Uint16(data[pos : pos+2])
		a.Class = binary.BigEndian.Uint16(data[pos+2 : pos+4])
		a.TTL = binary.BigEndian.Uint32(data[pos+4 : pos+8])
		dataLen := binary.BigEndian.Uint16(data[pos+8 : pos+10])
		pos += 10

		if a.Type == 12 { // PTR
			a.DomainName, read = dnsQueryDecode(data, pos)
			if read <= 0 {
				return fmt.Errorf("domain name: invalid string")
			}

			if uint16(read) != dataLen {
				return fmt.Errorf("read != dataLen, %d, %d", read, dataLen)
			}
		}

		pos += int(dataLen)
		l.Answers = append(l.Answers, a)
	}

	return nil
}

func (l *LayerMdns) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	data, err := b.AppendBytes(12)
	if err != nil {
		panic(err)
	}

	binary.BigEndian.PutUint16(data[0:2], l.TransactionId)
	if l.IsResponse {
		data[3] |= 0x01 << 7
	}
	data[3] |= l.Opcode << 3
	binary.BigEndian.PutUint16(data[4:6], uint16(len(l.Questions)))
	binary.BigEndian.PutUint16(data[6:8], uint16(len(l.Answers)))
	binary.BigEndian.PutUint16(data[8:10], l.AuthorityCount)
	binary.BigEndian.PutUint16(data[10:12], l.AdditionalCount)

	for _, q := range l.Questions {
		enc := dnsQueryEncode(q.Query)

		data, err := b.AppendBytes(len(enc) + 4)
		if err != nil {
			panic(err)
		}

		copy(data[:len(enc)], enc)
		data = data[len(enc):]
		binary.BigEndian.PutUint16(data[0:2], q.Type)
		binary.BigEndian.PutUint16(data[2:4], q.Class)
	}

	return nil
}
