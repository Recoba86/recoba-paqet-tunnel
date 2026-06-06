package buffer

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/xtaci/smux"
)

func TestTCPBridgeHTTPReturnsFullResponseBody(t *testing.T) {
	body := strings.Repeat("recoba-paqet-response-", 4096)
	response := exerciseHTTPBridge(t, body, false)

	if !strings.Contains(response, "HTTP/1.1 200 OK") {
		t.Fatalf("response missing status line: %q", response[:min(len(response), 128)])
	}
	if !strings.HasSuffix(response, body) {
		t.Fatalf("response body was not delivered in full: got %d bytes, want suffix body %d bytes", len(response), len(body))
	}
}

func TestTCPBridgePreservesResponseWhenRequestSideClosesFirst(t *testing.T) {
	body := strings.Repeat("request-side-finished-first-", 4096)
	response := exerciseHTTPBridge(t, body, true)

	if !strings.Contains(response, "Content-Length: ") {
		t.Fatalf("response missing headers: %q", response[:min(len(response), 128)])
	}
	if !strings.HasSuffix(response, body) {
		t.Fatalf("response body was lost after request-side CloseWrite: got %d bytes, want suffix body %d bytes", len(response), len(body))
	}
}

func TestTCPBridgeCompletesAfterBothDirectionsClose(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	leftClient, leftBridge := tcpPair(t)
	rightBridge, rightClient := tcpPair(t)
	defer leftClient.Close()
	defer rightClient.Close()

	done := make(chan error, 1)
	go func() {
		done <- CopyBidirectional(ctx, leftBridge, rightBridge, TCPBridgeOptions{
			AToB: TCPBridgeDirection{Name: "left-to-right", CloseWhenNoWriter: true},
			BToA: TCPBridgeDirection{Name: "right-to-left", CloseWhenNoWriter: true},
		})
	}()

	if _, err := leftClient.Write([]byte("ping")); err != nil {
		t.Fatalf("write through left side: %v", err)
	}

	buf := make([]byte, 4)
	if _, err := io.ReadFull(rightClient, buf); err != nil {
		t.Fatalf("read through right side: %v", err)
	}
	if string(buf) != "ping" {
		t.Fatalf("unexpected bridged payload %q", string(buf))
	}

	_ = leftClient.CloseWrite()
	_ = rightClient.CloseWrite()
	_ = leftClient.Close()
	_ = rightClient.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("bridge returned error after normal close: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("bridge did not complete after both directions closed")
	}
}

func exerciseHTTPBridge(t *testing.T, body string, closeRequestWrite bool) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientConn, forwardConn := tcpPair(t)
	defer clientConn.Close()

	backendConn, backendServer := tcpPair(t)
	defer backendConn.Close()

	streamForward, streamServer, closeStreams := smuxStreamPair(t)
	defer closeStreams()

	backendDone := make(chan error, 1)
	go func() {
		defer backendServer.Close()
		reader := bufio.NewReader(backendServer)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				backendDone <- fmt.Errorf("read request header: %w", err)
				return
			}
			if line == "\r\n" {
				break
			}
		}

		response := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
		if _, err := backendServer.Write([]byte(response)); err != nil {
			backendDone <- fmt.Errorf("write response: %w", err)
			return
		}
		if err := backendServer.CloseWrite(); err != nil {
			backendDone <- fmt.Errorf("close backend write side: %w", err)
			return
		}
		backendDone <- nil
	}()

	bridgeDone := make(chan error, 2)
	go func() {
		defer forwardConn.Close()
		defer streamForward.Close()
		bridgeDone <- CopyBidirectional(ctx, forwardConn, streamForward, TCPBridgeOptions{
			AToB: TCPBridgeDirection{Name: "local-to-stream"},
			BToA: TCPBridgeDirection{Name: "stream-to-local"},
		})
	}()
	go func() {
		defer streamServer.Close()
		defer backendConn.Close()
		bridgeDone <- CopyBidirectional(ctx, streamServer, backendConn, TCPBridgeOptions{
			AToB: TCPBridgeDirection{Name: "stream-to-backend"},
			BToA: TCPBridgeDirection{Name: "backend-to-stream", CloseWhenNoWriter: true},
		})
	}()

	request := "GET /1M.bin HTTP/1.1\r\nHost: 127.0.0.1\r\nUser-Agent: bridge-test\r\nAccept: */*\r\n\r\n"
	if _, err := clientConn.Write([]byte(request)); err != nil {
		t.Fatalf("write request: %v", err)
	}
	if closeRequestWrite {
		if err := clientConn.CloseWrite(); err != nil {
			t.Fatalf("close request write side: %v", err)
		}
	}

	responseBytes, err := io.ReadAll(clientConn)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}

	if err := clientConn.Close(); err != nil {
		t.Fatalf("close client connection: %v", err)
	}

	for i := 0; i < 2; i++ {
		select {
		case err := <-bridgeDone:
			if err != nil {
				t.Fatalf("bridge %d returned error: %v", i, err)
			}
		case <-ctx.Done():
			t.Fatal("bridge did not complete")
		}
	}

	select {
	case err := <-backendDone:
		if err != nil {
			t.Fatalf("backend failed: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("backend did not complete")
	}

	return string(responseBytes)
}

func tcpPair(t *testing.T) (*net.TCPConn, *net.TCPConn) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp pair: %v", err)
	}
	defer listener.Close()

	acceptDone := make(chan net.Conn, 1)
	acceptErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptErr <- err
			return
		}
		acceptDone <- conn
	}()

	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial tcp pair: %v", err)
	}

	var server net.Conn
	select {
	case err := <-acceptErr:
		client.Close()
		t.Fatalf("accept tcp pair: %v", err)
	case server = <-acceptDone:
	case <-time.After(2 * time.Second):
		client.Close()
		t.Fatal("timed out accepting tcp pair")
	}

	clientTCP, ok := client.(*net.TCPConn)
	if !ok {
		t.Fatalf("client connection type %T is not *net.TCPConn", client)
	}
	serverTCP, ok := server.(*net.TCPConn)
	if !ok {
		client.Close()
		server.Close()
		t.Fatalf("server connection type %T is not *net.TCPConn", server)
	}

	return clientTCP, serverTCP
}

func smuxStreamPair(t *testing.T) (net.Conn, net.Conn, func()) {
	t.Helper()

	clientTransport, serverTransport := tcpPair(t)
	clientSession, err := smux.Client(clientTransport, nil)
	if err != nil {
		clientTransport.Close()
		serverTransport.Close()
		t.Fatalf("create smux client session: %v", err)
	}
	serverSession, err := smux.Server(serverTransport, nil)
	if err != nil {
		clientSession.Close()
		t.Fatalf("create smux server session: %v", err)
	}

	acceptDone := make(chan *smux.Stream, 1)
	acceptErr := make(chan error, 1)
	go func() {
		stream, err := serverSession.AcceptStream()
		if err != nil {
			acceptErr <- err
			return
		}
		acceptDone <- stream
	}()

	clientStream, err := clientSession.OpenStream()
	if err != nil {
		clientSession.Close()
		serverSession.Close()
		t.Fatalf("open smux stream: %v", err)
	}

	var serverStream *smux.Stream
	select {
	case err := <-acceptErr:
		clientStream.Close()
		clientSession.Close()
		serverSession.Close()
		t.Fatalf("accept smux stream: %v", err)
	case serverStream = <-acceptDone:
	case <-time.After(2 * time.Second):
		clientStream.Close()
		clientSession.Close()
		serverSession.Close()
		t.Fatal("timed out accepting smux stream")
	}

	cleanup := func() {
		clientStream.Close()
		serverStream.Close()
		clientSession.Close()
		serverSession.Close()
	}

	return clientStream, serverStream, cleanup
}
