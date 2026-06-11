package socket

import (
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"paqet/internal/conf"
	"paqet/internal/pkg/iterator"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
)

func TestIsENOBUFS(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "libpcap text", err: errors.New("send: No buffer space available"), want: true},
		{name: "errno token", err: errors.New("write failed: ENOBUFS"), want: true},
		{name: "other error", err: errors.New("send: network is unreachable"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isENOBUFS(tt.err); got != tt.want {
				t.Fatalf("isENOBUFS() = %v, want %v", got, tt.want)
			}
		})
	}
}

type mockSocketHandle struct {
	writeDelay time.Duration
	err        error
	lastData   []byte
}

func (m *mockSocketHandle) WritePacketData(data []byte) error {
	m.lastData = make([]byte, len(data))
	copy(m.lastData, data)
	time.Sleep(m.writeDelay)
	return m.err
}

func (m *mockSocketHandle) Close() {}

func TestWritePacketData_ENOBUFSCap(t *testing.T) {
	mockErr := errors.New("enobufs") // isENOBUFS checks string content for "enobufs"
	mockHandle := &mockSocketHandle{err: mockErr}

	h := &SendHandle{
		handle: mockHandle,
		tx: conf.TX{
			RawPacketRetries: 100,   // Very high
			RawPacketRetryUS: 50000, // Very high
		},
	}

	start := time.Now()
	_ = h.writePacketData([]byte("test packet"))
	duration := time.Since(start)

	// Since we capped retries to 2 and max delay to 1000us (1ms),
	// the total delay should be very small (e.g. < 50ms)
	// If the cap didn't work, it would sleep for hundreds of seconds.
	if duration > 50*time.Millisecond {
		t.Fatalf("writePacketData took too long (%v), ENOBUFS retry cap failed", duration)
	}
}

func testSendHandle(t testing.TB) *SendHandle {
	if h, ok := t.(*testing.T); ok {
		h.Helper()
	}
	hw := net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	routerMAC := net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	srcIP := net.ParseIP("10.0.0.1")
	sh := &SendHandle{
		handle:      &mockSocketHandle{},
		tx:          conf.TX{RawPacketRetries: 1, RawPacketRetryUS: 100},
		srcPort:     443,
		srcIPv4:     srcIP,
		srcIPv4RHWA: routerMAC,
		time:        1000000,
		tcpF:        TCPF{tcpF: iterator.Iterator[conf.TCPF]{Items: []conf.TCPF{{ACK: true, PSH: true}}}, clientTCPF: make(map[uint64]*iterator.Iterator[conf.TCPF])},
		ethPool:     sync.Pool{New: func() any { return &layers.Ethernet{SrcMAC: hw} }},
		ipv4Pool:    sync.Pool{New: func() any { return &layers.IPv4{} }},
		ipv6Pool:    sync.Pool{New: func() any { return &layers.IPv6{} }},
		tcpPool:     sync.Pool{New: func() any { return &layers.TCP{} }},
		bufPool:     sync.Pool{New: func() any { return gopacket.NewSerializeBuffer() }},
		tsBufPool:   sync.Pool{New: func() any { b := make([]byte, 8); return &b }},
	}
	return sh
}

func TestPacketBuildRoundtrip(t *testing.T) {
	sh := testSendHandle(t)
	payload := []byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n")
	addr := &net.UDPAddr{IP: net.ParseIP("93.184.216.34"), Port: 80}

	// Build a packet via Write, capture the serialized bytes
	mock := sh.handle.(*mockSocketHandle)
	err := sh.Write(payload, addr)
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if mock.lastData == nil || len(mock.lastData) == 0 {
		t.Fatal("Write() did not produce packet data")
	}

	// Parse back with gopacket to verify structural correctness
	packet := gopacket.NewPacket(mock.lastData, layers.LayerTypeEthernet, gopacket.Default)
	ethLayer := packet.Layer(layers.LayerTypeEthernet)
	if ethLayer == nil {
		t.Fatal("no Ethernet layer in serialized packet")
	}
	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		t.Fatal("no IPv4 layer in serialized packet")
	}
	ip, _ := ipLayer.(*layers.IPv4)
	if !ip.DstIP.Equal(net.ParseIP("93.184.216.34")) {
		t.Fatalf("wrong dst IP: %s", ip.DstIP)
	}
	if !ip.SrcIP.Equal(net.ParseIP("10.0.0.1")) {
		t.Fatalf("wrong src IP: %s", ip.SrcIP)
	}
	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	if tcpLayer == nil {
		t.Fatal("no TCP layer in serialized packet")
	}
	tcp, _ := tcpLayer.(*layers.TCP)
	if tcp.DstPort != 80 {
		t.Fatalf("wrong dst port: %d", tcp.DstPort)
	}
	// Verify IPv4 checksum is non-zero (gopacket validates checksums during parsing)
	if ip.Checksum == 0 {
		t.Fatal("IPv4 checksum is zero")
	}
	// Verify TCP checksum is non-zero
	if tcp.Checksum == 0 {
		t.Fatal("TCP checksum is zero")
	}
	// Verify payload is intact
	appLayer := packet.ApplicationLayer()
	if appLayer == nil {
		t.Fatal("no application layer in packet")
	}
	if string(appLayer.Payload()) != string(payload) {
		t.Fatalf("payload mismatch: got %q want %q", string(appLayer.Payload()), string(payload))
	}
}

func BenchmarkWriteSmall(b *testing.B) {
	sh := testSendHandle(nil)
	payload := make([]byte, 64)
	addr := &net.UDPAddr{IP: net.ParseIP("93.184.216.34"), Port: 80}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = sh.Write(payload, addr)
	}
}

func BenchmarkWriteMTU(b *testing.B) {
	sh := testSendHandle(nil)
	payload := make([]byte, 1400)
	addr := &net.UDPAddr{IP: net.ParseIP("93.184.216.34"), Port: 80}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = sh.Write(payload, addr)
	}
}

func BenchmarkSerializeOnly(b *testing.B) {
	sh := testSendHandle(nil)
	payload := make([]byte, 1400)
	addr := &net.UDPAddr{IP: net.ParseIP("93.184.216.34"), Port: 80}
	buf := gopacket.NewSerializeBuffer()
	ethLayer := &layers.Ethernet{SrcMAC: sh.ethPool.New().(*layers.Ethernet).SrcMAC, DstMAC: sh.srcIPv4RHWA, EthernetType: layers.EthernetTypeIPv4}
	ipv4 := &layers.IPv4{Version: 4, IHL: 5, TOS: 184, TTL: 64, Flags: layers.IPv4DontFragment, Protocol: layers.IPProtocolTCP, SrcIP: sh.srcIPv4, DstIP: addr.IP}
	tsBuf := make([]byte, 8)
	tcp := &layers.TCP{SrcPort: layers.TCPPort(sh.srcPort), DstPort: layers.TCPPort(uint16(addr.Port)), ACK: true, PSH: true, Window: 65535}
	tcp.Options = []layers.TCPOption{
		{OptionType: layers.TCPOptionKindNop},
		{OptionType: layers.TCPOptionKindNop},
		{OptionType: layers.TCPOptionKindTimestamps, OptionLength: 10, OptionData: tsBuf},
	}
	tcp.SetNetworkLayerForChecksum(ipv4)
	opts := gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf.Clear()
		_ = gopacket.SerializeLayers(buf, opts, ethLayer, ipv4, tcp, gopacket.Payload(payload))
	}
}
