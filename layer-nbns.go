package main

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

const nbnsPort = 137

var layerTypeNbns gopacket.LayerType

type layerNbns struct {
	layers.BaseLayer
	TransactionID   uint16
	IsResponse      bool
	Opcode          uint8
	Truncated       bool
	Recursion       bool
	Broadcast       bool
	Questions       []nbnsQuestion
	Answers         []nbnsAnswer
	AuthorityCount  uint16
	AdditionalCount uint16
}

type nbnsQuestion struct {
	Query string
	Type  uint16
	Class uint16
}

type nbnsAnswer struct {
	Query string
	Type  uint16
	Class uint16
	TTL   uint32
	Names []nbnsAnswerName
}

type nbnsAnswerName struct {
	Name  string
	Type  uint8
	Flags uint16
}

func layerNbnsInit() {
	layerTypeNbns = gopacket.RegisterLayerType(
		2500,
		gopacket.LayerTypeMetadata{
			Name:    "Nbns",
			Decoder: gopacket.DecodeFunc(layerNbnsDecode),
		},
	)
	layers.RegisterUDPPortLayerType(nbnsPort, layerTypeNbns)
}

func layerNbnsDecode(data []byte, p gopacket.PacketBuilder) error {
	l := &layerNbns{}
	err := l.DecodeFromBytes(data, p)
	if err != nil {
		return err
	}
	p.AddLayer(l)
	p.SetApplicationLayer(l)
	return nil
}

func (l *layerNbns) LayerType() gopacket.LayerType {
	return layerTypeNbns
}

func (l *layerNbns) CanDecode() gopacket.LayerClass {
	return layerTypeNbns
}

func (l *layerNbns) NextLayerType() gopacket.LayerType {
	return gopacket.LayerTypeZero
}

func (l *layerNbns) Payload() []byte {
	return nil
}

func (l *layerNbns) DecodeFromBytes(data []byte, _ gopacket.DecodeFeedback) error {
	l.BaseLayer = layers.BaseLayer{Contents: data}

	if len(data) < 12 {
		return fmt.Errorf("invalid packet")
	}

	l.TransactionID = binary.BigEndian.Uint16(data[0:2])
	l.IsResponse = (data[3] >> 7) == 0x01
	l.Opcode = (data[3] >> 3) & 0x0F
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
		a := nbnsAnswer{}

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
			nameCount := data[pos2]
			pos2++

			for j := uint8(0); j < nameCount; j++ {
				a.Names = append(a.Names, nbnsAnswerName{
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

func (l *layerNbns) SerializeTo(b gopacket.SerializeBuffer, _ gopacket.SerializeOptions) error {
	data, err := b.AppendBytes(12)
	if err != nil {
		panic(err)
	}

	binary.BigEndian.PutUint16(data[0:2], l.TransactionID)
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
