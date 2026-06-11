package client

import (
	"context"
	"fmt"
	"paqet/internal/conf"
	"paqet/internal/protocol"
	"paqet/internal/socket"
	"paqet/internal/tnet"
	"paqet/internal/tnet/kcp"
	"sync/atomic"
	"time"
)

type timedConn struct {
	id     int
	cfg    *conf.Conf
	conn   tnet.Conn
	expire time.Time
	ctx    context.Context

	streamsOpened     uint64
	streamsFailed     uint64
	reconnectCount    uint64
	pingsFailed       uint64
	consecutiveErrors uint64

	degradedUntil atomic.Int64
}

const (
	degradedStreamFailDuration = 15 * time.Second
	degradedReconnectDuration  = 5 * time.Second
)

var nextConnID atomic.Int32

func newTimedConn(ctx context.Context, cfg *conf.Conf) (*timedConn, error) {
	var err error
	id := int(nextConnID.Add(1))
	tc := timedConn{cfg: cfg, ctx: ctx, id: id}
	tc.conn, err = tc.createConn()
	if err != nil {
		return nil, err
	}

	return &tc, nil
}

func (tc *timedConn) createConn() (tnet.Conn, error) {
	netCfg := tc.cfg.Network
	pConn, err := socket.New(tc.ctx, &netCfg)
	if err != nil {
		return nil, fmt.Errorf("could not create packet conn: %w", err)
	}

	conn, err := kcp.Dial(tc.cfg.Server.Addr, tc.cfg.Transport.KCP, pConn)
	if err != nil {
		return nil, err
	}
	err = tc.sendTCPF(conn)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (tc *timedConn) sendTCPF(conn tnet.Conn) error {
	strm, err := conn.OpenStrm()
	if err != nil {
		return err
	}
	defer strm.Close()

	p := protocol.Proto{Type: protocol.PTCPF, TCPF: tc.cfg.Network.TCP.RF}
	err = p.Write(strm)
	if err != nil {
		return err
	}
	return nil
}

func (tc *timedConn) close() {
	if tc.conn != nil {
		tc.conn.Close()
	}
}

func (tc *timedConn) isDegraded() bool {
	return time.Now().UnixNano() < tc.degradedUntil.Load()
}

func (tc *timedConn) markDegraded(d time.Duration) {
	tc.degradedUntil.Store(time.Now().Add(d).UnixNano())
}

func (tc *timedConn) recordStreamOpened() {
	atomic.AddUint64(&tc.streamsOpened, 1)
	atomic.StoreUint64(&tc.consecutiveErrors, 0)
}

func (tc *timedConn) recordStreamFailed() {
	atomic.AddUint64(&tc.streamsFailed, 1)
	c := atomic.AddUint64(&tc.consecutiveErrors, 1)
	if c >= 2 {
		tc.markDegraded(degradedStreamFailDuration)
	}
}

func (tc *timedConn) recordReconnect() {
	atomic.AddUint64(&tc.reconnectCount, 1)
	tc.markDegraded(degradedReconnectDuration)
}

func (tc *timedConn) recordPingFailed() { atomic.AddUint64(&tc.pingsFailed, 1) }

func (tc *timedConn) statsString() string {
	return fmt.Sprintf("conn=%d streams_open=%d streams_fail=%d reconnects=%d pings_fail=%d degraded=%v",
		tc.id,
		atomic.LoadUint64(&tc.streamsOpened),
		atomic.LoadUint64(&tc.streamsFailed),
		atomic.LoadUint64(&tc.reconnectCount),
		atomic.LoadUint64(&tc.pingsFailed),
		tc.isDegraded(),
	)
}
