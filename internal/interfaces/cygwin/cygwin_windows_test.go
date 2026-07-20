//go:build windows

package cygwin

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/sys/windows"
)

type extendedKeyring struct{ agent.Agent }

func (k extendedKeyring) SignWithFlags(key ssh.PublicKey, data []byte, _ agent.SignatureFlags) (*ssh.Signature, error) {
	return k.Sign(key, data)
}
func (k extendedKeyring) Extension(string, []byte) ([]byte, error) {
	return nil, agent.ErrExtensionUnsupported
}

func TestNonceRoundTrip(t *testing.T) {
	nonce := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	text := "!<socket >123 s " + nonceText(nonce)
	got, ok := parseSocketNonce(text)
	if !ok || !bytes.Equal(got, nonce) {
		t.Fatalf("%x %v", got, ok)
	}
}

func TestIntegrationHandshakeListAndCleanup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.sock")
	component := New(extendedKeyring{agent.NewKeyring()}, path, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- component.Start(ctx) }()
	var description string
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			description = string(data)
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if description == "" {
		cancel()
		t.Fatal("socket description was not created")
	}
	socketPath, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	attributes, err := windows.GetFileAttributes(socketPath)
	if err != nil {
		t.Fatal(err)
	}
	requiredAttributes := uint32(windows.FILE_ATTRIBUTE_SYSTEM | windows.FILE_ATTRIBUTE_READONLY)
	if attributes&requiredAttributes != requiredAttributes {
		t.Fatalf("socket attributes=%#x, want SYSTEM|READONLY", attributes)
	}
	fields := strings.Fields(description)
	port, err := strconv.Atoi(strings.TrimPrefix(fields[1], ">"))
	if err != nil {
		t.Fatal(err)
	}
	nonce, ok := parseSocketNonce(description)
	if !ok {
		t.Fatal("invalid nonce")
	}
	conn, err := net.Dial("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range [][]byte{nonce[:3], nonce[3:10], nonce[10:]} {
		if _, err := conn.Write(part); err != nil {
			t.Fatal(err)
		}
	}
	gotNonce := make([]byte, 16)
	if _, err := io.ReadFull(conn, gotNonce); err != nil || !bytes.Equal(gotNonce, nonce) {
		t.Fatalf("nonce=%x err=%v", gotNonce, err)
	}
	pidBlock := make([]byte, 12)
	if _, err := conn.Write(pidBlock[:5]); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(pidBlock[5:]); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(conn, pidBlock); err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint32(pidBlock[:4]) == 0 {
		t.Fatal("server PID missing")
	}
	keys, err := agent.NewClient(conn).List()
	if err != nil || len(keys) != 0 {
		t.Fatalf("keys=%v err=%v", keys, err)
	}
	_ = conn.Close()

	var wg sync.WaitGroup
	clientErrors := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			parallelConn, err := net.Dial("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
			if err != nil {
				clientErrors <- err
				return
			}
			defer parallelConn.Close()
			if _, err := parallelConn.Write(nonce); err != nil {
				clientErrors <- err
				return
			}
			reply := make([]byte, 16)
			if _, err := io.ReadFull(parallelConn, reply); err != nil {
				clientErrors <- err
				return
			}
			if _, err := parallelConn.Write(make([]byte, 12)); err != nil {
				clientErrors <- err
				return
			}
			if _, err := io.ReadFull(parallelConn, make([]byte, 12)); err != nil {
				clientErrors <- err
				return
			}
			_, err = agent.NewClient(parallelConn).List()
			clientErrors <- err
		}()
	}
	wg.Wait()
	close(clientErrors)
	for err := range clientErrors {
		if err != nil {
			t.Fatal(err)
		}
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("component did not stop")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("socket description remains")
	}
	if _, err := os.Stat(path + ".owner"); !os.IsNotExist(err) {
		t.Fatal("owner marker remains")
	}
}

func TestExistingUnownedFilesArePreserved(t *testing.T) {
	for _, test := range []struct {
		name      string
		makePath  func(string) error
		makeOwner bool
	}{
		{"regular file", func(path string) error { return os.WriteFile(path, []byte("notes"), 0600) }, false},
		{"directory", func(path string) error { return os.Mkdir(path, 0700) }, false},
		{"untrusted owner", func(path string) error {
			return os.WriteFile(path, []byte("!<socket >1 s 03020100-07060504-0b0a0908-0f0e0d0c"), 0600)
		}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "agent.sock")
			if err := test.makePath(path); err != nil {
				t.Fatal(err)
			}
			if test.makeOwner {
				if err := os.WriteFile(path+".owner", []byte(`{"version":1,"application":"Other","pid":0,"nonce":"000102030405060708090a0b0c0d0e0f"}`), 0600); err != nil {
					t.Fatal(err)
				}
			}
			component := New(nil, path, time.Second, nil)
			if err := component.prepareExisting(); err == nil {
				t.Fatal("expected conflict")
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("existing path was removed: %v", err)
			}
		})
	}
}

func TestOwnedStaleFilesAreRemoved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.sock")
	nonce := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	if err := os.WriteFile(path, []byte("!<socket >1 s "+nonceText(nonce)), 0600); err != nil {
		t.Fatal(err)
	}
	marker := `{"version":1,"application":"OmniSSHAgent","pid":0,"nonce":"000102030405060708090a0b0c0d0e0f"}`
	if err := os.WriteFile(path+".owner", []byte(marker), 0600); err != nil {
		t.Fatal(err)
	}
	component := New(nil, path, time.Second, nil)
	if err := component.prepareExisting(); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []string{path, path + ".owner"} {
		if _, err := os.Stat(candidate); !os.IsNotExist(err) {
			t.Fatalf("%s still exists", candidate)
		}
	}
}

func TestHandshakeRejectsNonceTimeoutAndShortPID(t *testing.T) {
	nonce := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	tests := []struct {
		name   string
		client func(net.Conn)
	}{
		{"nonce mismatch", func(conn net.Conn) { _, _ = conn.Write(make([]byte, 16)) }},
		{"timeout", func(net.Conn) { time.Sleep(40 * time.Millisecond) }},
		{"short PID", func(conn net.Conn) {
			_, _ = conn.Write(nonce)
			reply := make([]byte, 16)
			_, _ = io.ReadFull(conn, reply)
			_, _ = conn.Write(make([]byte, 5))
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			component := New(nil, "unused", 20*time.Millisecond, nil)
			server, client := net.Pipe()
			done := make(chan struct{})
			go func() {
				component.handle(context.Background(), server, nonce)
				close(done)
			}()
			test.client(client)
			_ = client.Close()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("handshake did not terminate")
			}
		})
	}
}
