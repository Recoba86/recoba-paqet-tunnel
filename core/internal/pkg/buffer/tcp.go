package buffer

import (
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"time"

	"paqet/internal/flog"
)

var (
	tcpWriteRetries    int32 = 8
	tcpWriteRetryUS    int32 = 200
	tcpWriteRetryMaxUS int32 = 25000
)

var (
	TCPWriteEnobufsTotal  uint64
	TCPWriteRetryTotal    uint64
	TCPWriteRetrySuccess  uint64
	TCPWriteRetryFailed   uint64
)

func SetTCPWriteRetryConfig(retries, retryUS, retryMaxUS int) {
	if retries >= 0 {
		atomic.StoreInt32(&tcpWriteRetries, int32(retries))
	}
	if retryUS >= 0 {
		atomic.StoreInt32(&tcpWriteRetryUS, int32(retryUS))
	}
	if retryMaxUS >= 0 {
		atomic.StoreInt32(&tcpWriteRetryMaxUS, int32(retryMaxUS))
	}
}

func TCPWriteRetryConfig() (retries, retryUS, retryMaxUS int) {
	return int(atomic.LoadInt32(&tcpWriteRetries)),
		int(atomic.LoadInt32(&tcpWriteRetryUS)),
		int(atomic.LoadInt32(&tcpWriteRetryMaxUS))
}

func LogTCPWriteMetrics(reason string) {
	flog.Warnf("tcp_write metrics %s: enobufs_total=%d retry_total=%d retry_success=%d retry_failed=%d",
		reason,
		atomic.LoadUint64(&TCPWriteEnobufsTotal),
		atomic.LoadUint64(&TCPWriteRetryTotal),
		atomic.LoadUint64(&TCPWriteRetrySuccess),
		atomic.LoadUint64(&TCPWriteRetryFailed),
	)
}

func CopyT(dst io.Writer, src io.Reader) error {
	buf := make([]byte, TPool)

	for {
		nr, readErr := src.Read(buf)
		if nr > 0 {
			if err := writeFullWithENOBUFSRetry(dst, buf[:nr]); err != nil {
				return err
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
	}
}

func writeFullWithENOBUFSRetry(dst io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := writeWithENOBUFSRetry(dst, data)
		if n > 0 {
			data = data[n:]
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

func writeWithENOBUFSRetry(dst io.Writer, data []byte) (int, error) {
	n, err := dst.Write(data)
	if !isENOBUFSError(err) {
		return n, err
	}

	atomic.AddUint64(&TCPWriteEnobufsTotal, 1)

	retries := int(atomic.LoadInt32(&tcpWriteRetries))
	baseUS := int(atomic.LoadInt32(&tcpWriteRetryUS))
	maxUS := int(atomic.LoadInt32(&tcpWriteRetryMaxUS))

	for attempt := 1; attempt <= retries; attempt++ {
		atomic.AddUint64(&TCPWriteRetryTotal, 1)

		delayUS := baseUS << (attempt - 1)
		if delayUS > maxUS {
			delayUS = maxUS
		}
		time.Sleep(time.Duration(delayUS) * time.Microsecond)

		retryN, retryErr := dst.Write(data[n:])
		n += retryN
		if !isENOBUFSError(retryErr) {
			atomic.AddUint64(&TCPWriteRetrySuccess, 1)
			LogTCPWriteMetrics("recovered ENOBUFS")
			return n, retryErr
		}
		if n >= len(data) {
			atomic.AddUint64(&TCPWriteRetrySuccess, 1)
			return n, nil
		}
	}

	atomic.AddUint64(&TCPWriteRetryFailed, 1)
	LogTCPWriteMetrics("failed to recover ENOBUFS")
	return n, err
}

func isENOBUFSError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no buffer space available") || strings.Contains(msg, "enobufs")
}
