package main

import (
	"encoding/binary"
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"strings"
)

const NBNS_PORT = 137

type LayerNbns struct {
	layers.BaseLayer
	TransactionId   uint16
	IsResponse      bool
	Opcode          uint8
	Truncated       bool
	Recursion       bool
	Broadcast       bool
	Questions       []NbnsQuestion
	Answers         []NbnsAnswer
	AuthorityCount  uint16
	AdditionalCount uint16
}

type NbnsQuestion struct {
	Query string
	Type  uint16
	Class uint16
}

type NbnsAnswer struct {
	Query string
	Type  uint16
	Class uint16
	TTL   uint32
	Names []NbnsAnswerName
}

type NbnsAnswerName struct {
	Name  string
	Type  uint8
	Flags uint16
}

var LayerTypeNbns gopacket.LayerType

func LayerNbnsInit() {
	LayerTypeNbns = gopacket.RegisterLayerType(
		2500,
		gopacket.LayerTypeMetadata{
			Name:    "Nbns",
			Decoder: gopacket.DecodeFunc(decodeLayerNbns),
		},
	)
	layers.RegisterUDPPortLayerType(NBNS_PORT, LayerTypeNbns)
}

func decodeLayerNbns(data []byte, p gopacket.PacketBuilder) error {
	l := &LayerNbns{}
	err := l.DecodeFromBytes(data, p)
	if err != nil {
		return err
	}
	p.AddLayer(l)
	p.SetApplicationLayer(l)
	return nil
}

func (l *LayerNbns) LayerType() gopacket.LayerType {
	return LayerTypeNbns
}

func (l *LayerNbns) CanDecode() gopacket.LayerClass {
	return LayerTypeNbns
}

func (l *LayerNbns) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypeZero
}

func (l *LayerNbns) Payload() []byte {
	return nil
}

func (l *LayerNbns) DecodeFromBytes(data []byte, df gopacket.DecodeFeedback) error {
	l.BaseLayer = layers.BaseLayer{Contents: data[:]}

	if len(data) < 12 {
		return fmt.Errorf("invalid packet")
	}

	l.TransactionId = binary.BigEndian.Uint16(data[0:2])
	l.IsResponse = (data[3] >> 7) == 0x01
	l.Opcode = uint8((data[3] >> 3) & 0x0F)
	l.Truncated = (data[3] >> 1) == 0x01
	l.Recursion = data[3] == 0x01
	l.Broadcast = (data[4] >> 4) == 0x01
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
		a := NbnsAnswer{}

		var read int
		a.Query, read = dnsQueryDecode(data, pos)
		if read <= 0 {
			return fmt.Errorf("invalid string")
		}
		pos += read

		a.Type = binary.BigEndian.Uint16(data[pos+0 : pos+2])
		a.Class = binary.BigEndian.Uint16(data[pos+2 : pos+4])
		a.TTL = binary.BigEndian.Uint32(data[pos+4 : pos+8])
		dataLen := binary.BigEndian.Uint16(data[pos+8 : pos+10])
		pos += 10

		if a.Type == 0x21 { // NB_STAT
			pos2 := pos
			nameCount := uint8(data[pos2])
			pos2++

			for j := uint8(0); j < nameCount; j++ {
				a.Names = append(a.Names, NbnsAnswerName{
					Name:  strings.TrimSuffix(string(data[pos2:pos2+15]), " "),
					Type:  data[pos2+15],
					Flags: binary.BigEndian.Uint16(data[pos2+16 : pos2+18]),
				})
				pos2 += 18
			}
		}

		pos += int(dataLen)
		l.Answers = append(l.Answers, a)
	}

	return nil
}

func (l *LayerNbns) SerializeTo(b gopacket.SerializeBuffer, opts gopacket.SerializeOptions) error {
	data, err := b.AppendBytes(12)
	if err != nil {
		panic(err)
	}

	binary.BigEndian.PutUint16(data[0:2], l.TransactionId)
	if l.IsResponse {
		data[3] |= 0x01 << 7
	}
	data[3] |= l.Opcode << 3
	if l.Truncated {
		data[3] |= 0x01 << 1
	}
	if l.Recursion {
		data[3] |= 0x01
	}
	if l.Broadcast {
		data[4] |= 0x01 << 4
	}
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
