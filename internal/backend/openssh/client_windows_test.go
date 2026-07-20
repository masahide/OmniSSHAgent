//go:build windows

package openssh

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type trackedConn struct {
	io.ReadWriter
	closed bool
	mu     sync.Mutex
}

func (c *trackedConn) Close() error { c.mu.Lock(); c.closed = true; c.mu.Unlock(); return nil }

func TestEveryOperationDialsAndCloses(t *testing.T) {
	var connections []*trackedConn
	dial := func(context.Context, string) (io.ReadWriteCloser, error) {
		server, client := netPipe()
		conn := &trackedConn{ReadWriter: client}
		connections = append(connections, conn)
		go func() {
			defer server.Close()
			_ = agent.ServeAgent(agent.NewKeyring(), server)
		}()
		return conn, nil
	}
	c := NewWithDialer("test", time.Second, dial)
	if _, err := c.List(); err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveAll(); err != nil {
		t.Fatal(err)
	}
	if len(connections) != 2 {
		t.Fatalf("dials=%d", len(connections))
	}
	for _, conn := range connections {
		if !conn.closed {
			t.Fatal("connection not closed")
		}
	}
}

func netPipe() (io.ReadWriteCloser, io.ReadWriteCloser) {
	a, b := io.Pipe()
	c, d := io.Pipe()
	return &pipeConn{Reader: a, Writer: d}, &pipeConn{Reader: c, Writer: b}
}

type pipeConn struct {
	io.Reader
	io.Writer
}

func (p *pipeConn) Close() error {
	if c, ok := p.Reader.(io.Closer); ok {
		_ = c.Close()
	}
	if c, ok := p.Writer.(io.Closer); ok {
		_ = c.Close()
	}
	return nil
}

func TestNormalizePipePath(t *testing.T) {
	if got := NormalizePipePath("agent"); got != `\\.\pipe\agent` {
		t.Fatal(got)
	}
	if got := NormalizePipePath(`\\.\pipe\agent`); got != `\\.\pipe\agent` {
		t.Fatal(got)
	}
}

type extensionAgent struct{ agent.ExtendedAgent }

func (a extensionAgent) Extension(extensionType string, contents []byte) ([]byte, error) {
	return append([]byte(extensionType+":"), contents...), nil
}

func TestForwardsExtendedAgentOperations(t *testing.T) {
	keyring := agent.NewKeyring().(agent.ExtendedAgent)
	serverAgent := extensionAgent{keyring}
	var mu sync.Mutex
	var connections []*trackedConn
	dial := func(context.Context, string) (io.ReadWriteCloser, error) {
		server, client := netPipe()
		conn := &trackedConn{ReadWriter: client}
		mu.Lock()
		connections = append(connections, conn)
		mu.Unlock()
		go func() {
			defer server.Close()
			_ = agent.ServeAgent(serverAgent, server)
		}()
		return conn, nil
	}
	client := NewWithDialer("test", time.Second, dial)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Add(agent.AddedKey{PrivateKey: private, Comment: "test"}); err != nil {
		t.Fatal(err)
	}
	keys, err := client.List()
	if err != nil || len(keys) != 1 {
		t.Fatalf("keys=%v err=%v", keys, err)
	}
	sshPublic, err := ssh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Sign(sshPublic, []byte("message")); err != nil {
		t.Fatal(err)
	}
	if _, err := client.SignWithFlags(sshPublic, []byte("message"), 0); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Signers(); err != nil {
		t.Fatal(err)
	}
	response, err := client.Extension("test-extension", []byte("body"))
	if err != nil || string(response) != "test-extension:body" {
		t.Fatalf("response=%q err=%v", response, err)
	}
	if err := client.Lock([]byte("passphrase")); err != nil {
		t.Fatal(err)
	}
	if err := client.Unlock([]byte("passphrase")); err != nil {
		t.Fatal(err)
	}
	if err := client.Remove(sshPublic); err != nil {
		t.Fatal(err)
	}
	if err := client.Add(agent.AddedKey{PrivateKey: private}); err != nil {
		t.Fatal(err)
	}
	if err := client.RemoveAll(); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(connections) != 11 {
		t.Fatalf("dials=%d", len(connections))
	}
	for _, conn := range connections {
		if !conn.closed {
			t.Fatal("connection not closed")
		}
	}
}

func TestTimeoutAndRecovery(t *testing.T) {
	var attempts int
	dial := func(ctx context.Context, _ string) (io.ReadWriteCloser, error) {
		attempts++
		if attempts == 1 {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		server, client := netPipe()
		go func() {
			defer server.Close()
			_ = agent.ServeAgent(extensionAgent{agent.NewKeyring().(agent.ExtendedAgent)}, server)
		}()
		return client, nil
	}
	client := NewWithDialer("test", 10*time.Millisecond, dial)
	if _, err := client.List(); err == nil {
		t.Fatal("expected timeout")
	}
	if _, err := client.List(); err != nil {
		t.Fatalf("recovery failed: %v", err)
	}
}

func TestBrokenConnectionDoesNotPoisonNextRequest(t *testing.T) {
	var attempts int
	client := NewWithDialer("test", time.Second, func(context.Context, string) (io.ReadWriteCloser, error) {
		attempts++
		server, peer := netPipe()
		if attempts == 1 {
			_ = server.Close()
			return peer, nil
		}
		go func() {
			defer server.Close()
			_ = agent.ServeAgent(extensionAgent{agent.NewKeyring().(agent.ExtendedAgent)}, server)
		}()
		return peer, nil
	})
	if _, err := client.List(); err == nil {
		t.Fatal("expected broken connection error")
	}
	if _, err := client.List(); err != nil {
		t.Fatal(err)
	}
}

func TestParallelRequestsUseIndependentConnections(t *testing.T) {
	var dials int
	var mu sync.Mutex
	client := NewWithDialer("test", time.Second, func(context.Context, string) (io.ReadWriteCloser, error) {
		mu.Lock()
		dials++
		mu.Unlock()
		server, peer := netPipe()
		go func() {
			defer server.Close()
			_ = agent.ServeAgent(extensionAgent{agent.NewKeyring().(agent.ExtendedAgent)}, server)
		}()
		return peer, nil
	})
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.List()
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil && !errors.Is(err, io.EOF) {
			t.Fatal(err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if dials != 8 {
		t.Fatalf("dials=%d", dials)
	}
}

func TestNamedPipeIntegrationStopAndRecovery(t *testing.T) {
	path := fmt.Sprintf(`\\.\pipe\OmniSSHAgent-Test-%d`, os.Getpid())
	serveOne := func() (<-chan error, func()) {
		listener, err := winio.ListenPipe(path, nil)
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			conn, err := listener.Accept()
			if err == nil {
				err = agent.ServeAgent(extensionAgent{agent.NewKeyring().(agent.ExtendedAgent)}, conn)
				_ = conn.Close()
			}
			done <- err
		}()
		return done, func() { _ = listener.Close() }
	}

	client := New(path, 30*time.Millisecond)
	done, closeServer := serveOne()
	if _, err := client.List(); err != nil {
		t.Fatal(err)
	}
	closeServer()
	<-done
	if _, err := client.List(); err == nil {
		t.Fatal("expected request failure while pipe is stopped")
	}
	done, closeServer = serveOne()
	if _, err := client.List(); err != nil {
		t.Fatalf("request did not recover: %v", err)
	}
	closeServer()
	<-done
}
