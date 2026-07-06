package service

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestSOCKS5ProxyDialContextHonorsRequestContext(t *testing.T) {
	ResetProxyClientCache()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer listener.Close()

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			accepted <- conn
		}
	}()
	defer func() {
		select {
		case conn := <-accepted:
			conn.Close()
		default:
		}
	}()

	client, err := NewProxyHttpClient("socks5://user:pass@" + listener.Addr().String())
	if err != nil {
		t.Fatalf("NewProxyHttpClient returned error: %v", err)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", client.Transport)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	type dialResult struct {
		conn net.Conn
		err  error
	}
	result := make(chan dialResult, 1)
	startedAt := time.Now()
	go func() {
		conn, err := transport.DialContext(ctx, "tcp", "example.com:443")
		result <- dialResult{conn: conn, err: err}
	}()

	select {
	case result := <-result:
		elapsed := time.Since(startedAt)
		if result.conn != nil {
			result.conn.Close()
		}
		if !isTimeoutOrDeadline(result.err) {
			t.Fatalf("DialContext error = %v, want timeout/deadline error", result.err)
		}
		if elapsed > 150*time.Millisecond {
			t.Fatalf("DialContext returned after %s, want it to respect request context deadline", elapsed)
		}
	case <-time.After(200 * time.Millisecond):
		listener.Close()
		select {
		case conn := <-accepted:
			conn.Close()
		default:
		}
		t.Fatal("DialContext did not return after request context deadline")
	}
}

func isTimeoutOrDeadline(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}
