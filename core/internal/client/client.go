package client

import (
	"context"
	"paqet/internal/conf"
	"paqet/internal/flog"
	"paqet/internal/pkg/iterator"
	"paqet/internal/tnet"
	"strings"
	"sync"
	"sync/atomic"
)

type Client struct {
	cfg     *conf.Conf
	iter    *iterator.Iterator[*timedConn]
	udpPool *udpPool
	mu      sync.Mutex

	totalStreamsOpened atomic.Uint64

	newStrmOverride func() (tnet.Strm, error)
}

func New(cfg *conf.Conf) (*Client, error) {
	c := &Client{
		cfg:  cfg,
		iter: &iterator.Iterator[*timedConn]{},
		udpPool: &udpPool{
			strms:   make(map[uint64]tnet.Strm),
			pending: make(map[uint64]chan struct{}),
		},
	}
	return c, nil
}

func (c *Client) Start(ctx context.Context) error {
	for i := range c.cfg.Transport.Conn {
		tc, err := newTimedConn(ctx, c.cfg)
		if err != nil {
			flog.Errorf("failed to create connection %d: %v", i+1, err)
			return err
		}
		flog.Infof("client connection %d created successfully (id=%d)", i+1, tc.id)
		c.iter.Items = append(c.iter.Items, tc)
	}
	go c.ticker(ctx)

	go func() {
		<-ctx.Done()
		for _, tc := range c.iter.Items {
			tc.close()
		}
		flog.Infof("client shutdown complete")
	}()

	ipv4Addr := "<nil>"
	ipv6Addr := "<nil>"
	if c.cfg.Network.IPv4.Addr != nil {
		ipv4Addr = c.cfg.Network.IPv4.Addr.IP.String()
	}
	if c.cfg.Network.IPv6.Addr != nil {
		ipv6Addr = c.cfg.Network.IPv6.Addr.IP.String()
	}
	flog.Infof("Client started: IPv4:%s IPv6:%s -> %s (%d connections)", ipv4Addr, ipv6Addr, c.cfg.Server.Addr, len(c.iter.Items))
	return nil
}

func (c *Client) connStats() string {
	var parts []string
	for _, tc := range c.iter.Items {
		parts = append(parts, tc.statsString())
	}
	return strings.Join(parts, "; ")
}
