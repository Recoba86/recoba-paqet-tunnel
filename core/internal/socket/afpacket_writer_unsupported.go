//go:build !linux

package socket

import "fmt"

func newAFPacketWriter(ifaceName string) (PcapHandle, error) {
	return nil, fmt.Errorf("af_packet: not supported on this platform (Linux only)")
}
