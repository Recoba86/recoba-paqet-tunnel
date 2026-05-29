package buffer

import (
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
)

type flakyENOBUFSWriter struct {
	failures int
	written  strings.Builder
}

func (w *flakyENOBUFSWriter) Write(p []byte) (int, error) {
	if w.failures > 0 {
		w.failures--
		return 0, errors.New("send: No buffer space available")
	}
	return w.written.Write(p)
}

func resetTCPWriteMetrics() {
	atomic.StoreUint64(&TCPWriteEnobufsTotal, 0)
	atomic.StoreUint64(&TCPWriteRetryTotal, 0)
	atomic.StoreUint64(&TCPWriteRetrySuccess, 0)
	atomic.StoreUint64(&TCPWriteRetryFailed, 0)
}

func TestCopyTRetriesENOBUFS(t *testing.T) {
	Initialize(8, 8)
	SetTCPWriteRetryConfig(8, 200, 25000)
	resetTCPWriteMetrics()

	dst := &flakyENOBUFSWriter{failures: 2}

	if err := CopyT(dst, strings.NewReader("hello world")); err != nil {
		t.Fatalf("CopyT() error = %v", err)
	}
	if got := dst.written.String(); got != "hello world" {
		t.Fatalf("written data = %q, want %q", got, "hello world")
	}
	if n := atomic.LoadUint64(&TCPWriteEnobufsTotal); n != 1 {
		t.Fatalf("enobufs_total = %d, want 1", n)
	}
	if n := atomic.LoadUint64(&TCPWriteRetrySuccess); n != 1 {
		t.Fatalf("retry_success = %d, want 1", n)
	}
}

func TestCopyTReturnsPersistentENOBUFS(t *testing.T) {
	Initialize(8, 8)
	SetTCPWriteRetryConfig(3, 200, 25000)
	resetTCPWriteMetrics()

	dst := &flakyENOBUFSWriter{failures: 10}

	err := CopyT(dst, strings.NewReader("hello"))
	if err == nil {
		t.Fatal("CopyT() error = nil, want ENOBUFS")
	}
	if !isENOBUFSError(err) {
		t.Fatalf("CopyT() error = %v, want ENOBUFS", err)
	}
	if n := atomic.LoadUint64(&TCPWriteRetryFailed); n != 1 {
		t.Fatalf("retry_failed = %d, want 1", n)
	}
}

func TestCopyTHandlesEOF(t *testing.T) {
	Initialize(8, 8)
	resetTCPWriteMetrics()

	dst := &flakyENOBUFSWriter{}

	if err := CopyT(dst, strings.NewReader("")); err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("CopyT() error = %v, want nil", err)
	}
}

func TestCopyTRecoversWith8Retries(t *testing.T) {
	Initialize(8, 8)
	SetTCPWriteRetryConfig(8, 200, 25000)
	resetTCPWriteMetrics()

	dst := &flakyENOBUFSWriter{failures: 7}

	if err := CopyT(dst, strings.NewReader("data after many retries")); err != nil {
		t.Fatalf("CopyT() error = %v, want nil (7 failures should be recoverable with 8 retries)", err)
	}
	if n := atomic.LoadUint64(&TCPWriteRetrySuccess); n != 1 {
		t.Fatalf("retry_success = %d, want 1", n)
	}
	if n := atomic.LoadUint64(&TCPWriteRetryFailed); n != 0 {
		t.Fatalf("retry_failed = %d, want 0", n)
	}
}

func TestCopyTMaxDelayClamping(t *testing.T) {
	Initialize(8, 8)
	SetTCPWriteRetryConfig(4, 200, 500)
	resetTCPWriteMetrics()

	dst := &flakyENOBUFSWriter{failures: 3}

	if err := CopyT(dst, strings.NewReader("clamped delay test")); err != nil {
		t.Fatalf("CopyT() error = %v", err)
	}
	if n := atomic.LoadUint64(&TCPWriteRetrySuccess); n != 1 {
		t.Fatalf("retry_success = %d, want 1", n)
	}
}

func TestSetTCPWriteRetryConfig(t *testing.T) {
	SetTCPWriteRetryConfig(5, 300, 5000)
	r, us, maxUS := TCPWriteRetryConfig()
	if r != 5 || us != 300 || maxUS != 5000 {
		t.Fatalf("TCPWriteRetryConfig() = (%d, %d, %d), want (5, 300, 5000)", r, us, maxUS)
	}

	SetTCPWriteRetryConfig(-1, -1, -1)
	r, us, maxUS = TCPWriteRetryConfig()
	if r != 5 {
		t.Fatalf("negative values should not overwrite: retries = %d, want 5", r)
	}
	if us != 300 {
		t.Fatalf("negative values should not overwrite: retry_us = %d, want 300", us)
	}
	if maxUS != 5000 {
		t.Fatalf("negative values should not overwrite: retry_max_us = %d, want 5000", maxUS)
	}
}
