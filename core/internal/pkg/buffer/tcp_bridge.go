package buffer

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
)

type TCPBridgeOptions struct {
	AToB TCPBridgeDirection
	BToA TCPBridgeDirection
}

type TCPBridgeDirection struct {
	Name              string
	CloseWhenNoWriter bool
}

type closeWriter interface {
	CloseWrite() error
}

type bridgeResult struct {
	name string
	err  error
}

func CopyBidirectional(ctx context.Context, a, b net.Conn, opts TCPBridgeOptions) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan bridgeResult, 2)
	var closeOnce sync.Once
	closeBoth := func() {
		closeOnce.Do(func() {
			_ = a.Close()
			_ = b.Close()
		})
	}

	go copyBridgeDirection(results, opts.AToB, b, a)
	go copyBridgeDirection(results, opts.BToA, a, b)

	var hardErr error
	completed := 0
	for completed < 2 {
		select {
		case result := <-results:
			completed++
			if result.err != nil && !isNormalTCPBridgeClose(result.err) && hardErr == nil {
				hardErr = result.err
				closeBoth()
			}
		case <-ctx.Done():
			if hardErr == nil && !errors.Is(ctx.Err(), context.Canceled) {
				hardErr = ctx.Err()
			}
			closeBoth()
			for completed < 2 {
				result := <-results
				completed++
				if result.err != nil && !isNormalTCPBridgeClose(result.err) && hardErr == nil {
					hardErr = result.err
				}
			}
		}
	}

	return hardErr
}

func copyBridgeDirection(results chan<- bridgeResult, direction TCPBridgeDirection, dst, src net.Conn) {
	name := direction.Name
	if name == "" {
		name = "tcp-bridge"
	}

	err := CopyT(dst, src)
	if err == nil {
		signalBridgeEOF(dst, direction.CloseWhenNoWriter)
	}

	results <- bridgeResult{name: name, err: err}
}

func signalBridgeEOF(conn net.Conn, closeWhenNoWriter bool) {
	if cw, ok := conn.(closeWriter); ok {
		_ = cw.CloseWrite()
		return
	}
	if closeWhenNoWriter {
		go func() {
			_ = conn.Close()
		}()
	}
}

func isNormalTCPBridgeClose(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.Is(err, io.ErrClosedPipe) {
		return true
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "use of closed network connection") ||
		strings.Contains(msg, "closed pipe")
}
