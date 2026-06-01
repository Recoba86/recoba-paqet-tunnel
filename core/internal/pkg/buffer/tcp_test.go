package buffer

import (
	"errors"
	"io"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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

type flakyNonENOBUFSWriter struct {
	failures int
}

func (w *flakyNonENOBUFSWriter) Write(p []byte) (int, error) {
	if w.failures > 0 {
		w.failures--
		return 0, errors.New("some other error")
	}
	return len(p), nil
}

type partialWriteENOBUFSWriter struct {
	writes  int
	written strings.Builder
}

func (w *partialWriteENOBUFSWriter) Write(p []byte) (int, error) {
	w.writes++
	// First write: partial write
	if w.writes == 1 {
		w.written.Write(p[:2])
		return 2, nil
	}
	// Second write: ENOBUFS
	if w.writes == 2 {
		return 0, errors.New("enobufs")
	}
	// Third write: rest of the data
	w.written.Write(p)
	return len(p), nil
}

func resetTCPWriteMetrics() {
	atomic.StoreUint64(&TCPWriteEnobufsTotal, 0)
	atomic.StoreUint64(&TCPWriteRetryTotal, 0)
	atomic.StoreUint64(&TCPWriteRetrySuccess, 0)
	atomic.StoreUint64(&TCPWriteRetryFailed, 0)
}

func TestCopyTRecoversWith12Retries(t *testing.T) {
	Initialize(8, 8)
	SetTCPWriteRetryConfig(12, 500, 100000)
	resetTCPWriteMetrics()

	dst := &flakyENOBUFSWriter{failures: 10}

	start := time.Now()
	if err := CopyT(dst, strings.NewReader("hello world")); err != nil {
		t.Fatalf("CopyT() error = %v", err)
	}
	elapsed := time.Since(start)

	if got := dst.written.String(); got != "hello world" {
		t.Fatalf("written data = %q, want %q", got, "hello world")
	}
	if n := atomic.LoadUint64(&TCPWriteRetrySuccess); n != 1 {
		t.Fatalf("retry_success = %d, want 1", n)
	}
	if n := atomic.LoadUint64(&TCPWriteRetryFailed); n != 0 {
		t.Fatalf("retry_failed = %d, want 0", n)
	}

	// Total backoff upper bound is reasonable check
	if elapsed > 3*time.Second {
		t.Fatalf("elapsed time too long: %v", elapsed)
	}
}

func TestCopyTReturnsPersistentENOBUFS(t *testing.T) {
	Initialize(8, 8)
	SetTCPWriteRetryConfig(12, 500, 100000)
	resetTCPWriteMetrics()

	dst := &flakyENOBUFSWriter{failures: 15}

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

func TestCopyTNonENOBUFS(t *testing.T) {
	Initialize(8, 8)
	SetTCPWriteRetryConfig(12, 500, 100000)
	resetTCPWriteMetrics()

	dst := &flakyNonENOBUFSWriter{failures: 1}

	err := CopyT(dst, strings.NewReader("hello"))
	if err == nil {
		t.Fatal("CopyT() error = nil, want some other error")
	}
	if isENOBUFSError(err) {
		t.Fatalf("CopyT() error = %v, want non-ENOBUFS", err)
	}
	// It should return immediately without consuming retries
	if n := atomic.LoadUint64(&TCPWriteEnobufsTotal); n != 0 {
		t.Fatalf("enobufs_total = %d, want 0", n)
	}
}

func TestCopyTPartialWriteENOBUFS(t *testing.T) {
	Initialize(8, 8)
	SetTCPWriteRetryConfig(12, 500, 100000)
	resetTCPWriteMetrics()

	dst := &partialWriteENOBUFSWriter{}

	if err := CopyT(dst, strings.NewReader("hello world")); err != nil {
		t.Fatalf("CopyT() error = %v", err)
	}
	if got := dst.written.String(); got != "hello world" {
		t.Fatalf("written data = %q, want %q", got, "hello world")
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

func TestCopyTNoGoroutineLeaks(t *testing.T) {
	Initialize(8, 8)
	SetTCPWriteRetryConfig(3, 10, 100)
	resetTCPWriteMetrics()

	initialGoroutines := runtime.NumGoroutine()

	dst := &flakyENOBUFSWriter{failures: 10}
	_ = CopyT(dst, strings.NewReader("leak test"))

	time.Sleep(50 * time.Millisecond) // Give time for any spawned goroutines to exit if they existed
	finalGoroutines := runtime.NumGoroutine()

	if finalGoroutines > initialGoroutines {
		t.Fatalf("goroutine leak: initial=%d, final=%d", initialGoroutines, finalGoroutines)
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
