package socket

import (
	"fmt"
	"os"
	"paqet/internal/conf"
	"paqet/internal/flog"
	"runtime"

	"github.com/gopacket/gopacket/pcap"
)

func newHandle(cfg *conf.Network) (*pcap.Handle, error) {
	// On Windows, use the GUID field to construct the NPF device name
	// On other platforms, use the interface name directly
	ifaceName := cfg.Interface.Name
	if runtime.GOOS == "windows" {
		ifaceName = cfg.GUID
	}

	inactive, err := pcap.NewInactiveHandle(ifaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to create inactive pcap handle for %s: %v", cfg.Interface.Name, err)
	}
	defer inactive.CleanUp()

	if err = inactive.SetBufferSize(cfg.PCAP.Sockbuf); err != nil {
		return nil, fmt.Errorf("failed to set pcap buffer size to %d: %v", cfg.PCAP.Sockbuf, err)
	}

	if err = inactive.SetSnapLen(65536); err != nil {
		return nil, fmt.Errorf("failed to set pcap snap length: %v", err)
	}
	if err = inactive.SetPromisc(true); err != nil {
		return nil, fmt.Errorf("failed to enable promiscuous mode: %v", err)
	}
	if err = inactive.SetTimeout(pcap.BlockForever); err != nil {
		return nil, fmt.Errorf("failed to set pcap timeout: %v", err)
	}
	if err = inactive.SetImmediateMode(true); err != nil {
		return nil, fmt.Errorf("failed to enable immediate mode: %v", err)
	}

	handle, err := inactive.Activate()
	if err != nil {
		return nil, fmt.Errorf("failed to activate pcap handle on %s: %v", cfg.Interface.Name, err)
	}

	return handle, nil
}

func newWriteHandle(cfg *conf.Network) (PcapHandle, error) {
	writerEnv := os.Getenv("RECOBA_PACKET_WRITER")
	if writerEnv == "afpacket" {
		w, err := newAFPacketWriter(cfg.Interface.Name)
		if err != nil {
			flog.Warnf("af_packet writer failed, falling back to pcap: %v", err)
		} else {
			flog.Infof("using AF_PACKET writer on interface %s (ifindex=%d)", cfg.Interface.Name, cfg.Interface.Index)
			return w, nil
		}
	}

	h, err := newHandle(cfg)
	if err != nil {
		return nil, err
	}
	if runtime.GOOS != "windows" {
		if err := h.SetDirection(pcap.DirectionOut); err != nil {
			return nil, fmt.Errorf("failed to set pcap direction out: %v", err)
		}
	}
	return h, nil
}
