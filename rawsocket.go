package main

import (
	"net"
	"syscall"

	"github.com/google/gopacket/pcapgo"
)

type rawSocket struct {
	reader *pcapgo.EthernetHandle
	writer int
}

func newRawSocket(intf *net.Interface) (*rawSocket, error) {
	reader, err := pcapgo.NewEthernetHandle(intf.Name)
	if err != nil {
		return nil, err
	}

	writer, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, syscall.ETH_P_ALL)
	if err != nil {
		return nil, err
	}

	err = syscall.Bind(writer, &syscall.SockaddrLinklayer{
		Protocol: syscall.ETH_P_IP,
		Ifindex:  intf.Index,
	})
	if err != nil {
		return nil, err
	}

	return &rawSocket{
		reader: reader,
		writer: writer,
	}, nil
}

func (s *rawSocket) Read() ([]byte, error) {
	byts, _, err := s.reader.ZeroCopyReadPacketData()
	return byts, err
}

func (s *rawSocket) Write(byts []byte) error {
	_, err := syscall.Write(s.writer, byts)
	return err
}
