package agentlistener

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestServeStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listener := newFakeListener()
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, listener, func(context.Context, net.Conn) {})
	}()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve did not return after context cancel")
	}
}

func TestServeHandlerInvokedBeforeShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	listener := newFakeListener()
	handlerCalled := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		done <- Serve(ctx, listener, func(ctx context.Context, conn net.Conn) {
			close(handlerCalled)
			conn.Close()
		})
	}()

	conn, remote := net.Pipe()
	defer remote.Close()
	listener.enqueue(conn)

	select {
	case <-handlerCalled:
	case <-time.After(time.Second):
		t.Fatal("handler was not invoked")
	}

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve did not return after context cancel")
	}
}

type fakeListener struct {
	conns  chan net.Conn
	closed chan struct{}
}

func newFakeListener() *fakeListener {
	return &fakeListener{
		conns:  make(chan net.Conn, 1),
		closed: make(chan struct{}),
	}
}

func (f *fakeListener) Accept() (net.Conn, error) {
	select {
	case conn := <-f.conns:
		return conn, nil
	case <-f.closed:
		return nil, net.ErrClosed
	}
}

func (f *fakeListener) Close() error {
	select {
	case <-f.closed:
	default:
		close(f.closed)
	}
	return nil
}

func (f *fakeListener) Addr() net.Addr {
	return fakeAddr("fake")
}

func (f *fakeListener) enqueue(conn net.Conn) {
	f.conns <- conn
}

type fakeAddr string

func (fakeAddr) Network() string { return "fake" }
func (fakeAddr) String() string  { return "fake" }
