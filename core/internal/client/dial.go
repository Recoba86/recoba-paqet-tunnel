package client

import (
	"fmt"
	"paqet/internal/flog"
	"paqet/internal/tnet"
	"time"
)

func (c *Client) pickConnLocked() (*timedConn, error) {
	items := c.iter.Items
	if len(items) == 0 {
		return nil, fmt.Errorf("no connections available")
	}
	if len(items) == 1 {
		_ = c.iter.Next()
		return items[0], nil
	}

	for i := 0; i < len(items); i++ {
		tc := c.iter.Next()
		if !tc.isDegraded() {
			return tc, nil
		}
		flog.Debugf("conn %d skipped (degraded)", tc.id)
	}

	flog.Infof("all %d conns degraded, falling back", len(items))
	return c.iter.Next(), nil
}

func (c *Client) newConn() (*timedConn, tnet.Conn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	autoExpire := 300

	tc, err := c.pickConnLocked()
	if err != nil {
		return nil, nil, err
	}

	flog.Debugf("new stream assigned to conn %d (streams_open=%d, degraded=%v)",
		tc.id, tc.streamsOpened, tc.isDegraded())
	err = tc.conn.Ping(false)
	if err != nil {
		flog.Infof("connection lost on conn %d, retrying....", tc.id)
		tc.recordPingFailed()
		if tc.conn != nil {
			tc.conn.Close()
		}
		if newC, err2 := tc.createConn(); err2 == nil {
			tc.conn = newC
			tc.recordReconnect()
			tc.markDegraded(degradedReconnectDuration)
			flog.Infof("conn %d reconnected successfully", tc.id)
		} else {
			flog.Errorf("conn %d failed to reconnect: %v", tc.id, err2)
			return nil, nil, fmt.Errorf("failed to recreate connection: %v", err)
		}
		tc.expire = time.Now().Add(time.Duration(autoExpire) * time.Second)
	}
	return tc, tc.conn, nil
}

func (c *Client) newStrm() (tnet.Strm, error) {
	if c.newStrmOverride != nil {
		return c.newStrmOverride()
	}

	var tc *timedConn
	var conn tnet.Conn
	var strm tnet.Strm
	var err error

	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			delay := time.Duration(100<<uint(attempt-1)) * time.Millisecond
			time.Sleep(delay)
		}

		tc, conn, err = c.newConn()
		if err != nil {
			flog.Debugf("session creation failed on attempt %d: %v", attempt+1, err)
			continue
		}

		strm, err = conn.OpenStrm()
		if err != nil {
			flog.Debugf("failed to open stream on attempt %d via conn %d: %v", attempt+1, tc.id, err)
			tc.recordStreamFailed()
			continue
		}

		tc.recordStreamOpened()
		c.totalStreamsOpened.Add(1)
		flog.Infof("stream %d opened successfully (total streams=%d) %s",
			strm.SID(), c.totalStreamsOpened.Load(), c.connStats())

		return strm, nil
	}

	return nil, fmt.Errorf("failed to open stream after 3 attempts: %w", err)
}
