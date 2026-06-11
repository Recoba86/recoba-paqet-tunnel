package socket

import (
	"encoding/binary"
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

type afPacketWriter struct {
	fd      int
	ifIndex int
}

func newAFPacketWriter(ifaceName string) (PcapHandle, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return nil, fmt.Errorf("af_packet: interface %s not found: %w", ifaceName, err)
	}

	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons(unix.ETH_P_ALL)))
	if err != nil {
		return nil, fmt.Errorf("af_packet: socket creation failed: %w", err)
	}

	addr := unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ALL),
		Ifindex:  iface.Index,
	}
	if err := unix.Bind(fd, &addr); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("af_packet: bind to ifindex %d failed: %w", iface.Index, err)
	}

	return &afPacketWriter{fd: fd, ifIndex: iface.Index}, nil
}

func (w *afPacketWriter) WritePacketData(data []byte) error {
	addr := unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ALL),
		Ifindex:  w.ifIndex,
	}
	return unix.Sendto(w.fd, data, 0, &addr)
}

func (w *afPacketWriter) Close() {
	unix.Close(w.fd)
}

func htons(v uint16) uint16 {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, v)
	return binary.BigEndian.Uint16(b)
}
