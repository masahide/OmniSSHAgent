//go:build windows

package cygwin

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/masahide/OmniSSHAgent/internal/backend"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/sys/windows"
)

const application = "OmniSSHAgent"

type ownerMarker struct {
	Version     int    `json:"version"`
	Application string `json:"application"`
	PID         int    `json:"pid"`
	Nonce       string `json:"nonce"`
}

type Component struct {
	backend          backend.Backend
	path             string
	handshakeTimeout time.Duration
	logger           *slog.Logger
	ready            chan error
	readyOnce        sync.Once
}

func New(b backend.Backend, path string, timeout time.Duration, logger *slog.Logger) *Component {
	return &Component{backend: b, path: path, handshakeTimeout: timeout, logger: logger, ready: make(chan error, 1)}
}
func (c *Component) Name() string          { return "cygwin" }
func (c *Component) Ready() <-chan error   { return c.ready }
func (c *Component) reportReady(err error) { c.readyOnce.Do(func() { c.ready <- err }) }

func (c *Component) Start(ctx context.Context) (resultErr error) {
	defer func() {
		if resultErr != nil {
			c.reportReady(resultErr)
		}
	}()
	if err := c.prepareExisting(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0700); err != nil {
		return fmt.Errorf("create socket directory: %w", err)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen on loopback: %w", err)
	}
	defer listener.Close()
	nonce := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("create nonce: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := c.writeFiles(port, nonce); err != nil {
		return err
	}
	defer c.cleanup(nonce)
	c.reportReady(nil)

	go func() {
		<-ctx.Done()
		listener.Close()
	}()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("accept Cygwin client: %w", err)
		}
		go c.handle(ctx, conn, nonce)
	}
}

func (c *Component) handle(ctx context.Context, conn net.Conn, nonce []byte) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(c.handshakeTimeout))
	got := make([]byte, 16)
	if _, err := io.ReadFull(conn, got); err != nil || !bytes.Equal(got, nonce) {
		return
	}
	if _, err := conn.Write(nonce); err != nil {
		return
	}
	pidData := make([]byte, 12)
	if _, err := io.ReadFull(conn, pidData); err != nil {
		return
	}
	binary.LittleEndian.PutUint32(pidData[:4], uint32(os.Getpid()))
	if _, err := conn.Write(pidData); err != nil {
		return
	}
	_ = conn.SetDeadline(time.Time{})
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-done:
		}
	}()
	err := agent.ServeAgent(c.backend, conn)
	close(done)
	if err != nil && !errors.Is(err, io.EOF) && c.logger != nil {
		c.logger.Warn("Cygwin client request failed", "error", err)
	}
}

func nonceText(nonce []byte) string {
	parts := make([]string, 0, 4)
	for i := 0; i < 16; i += 4 {
		parts = append(parts, hex.EncodeToString([]byte{nonce[i+3], nonce[i+2], nonce[i+1], nonce[i]}))
	}
	return strings.Join(parts, "-")
}

func (c *Component) writeFiles(port int, nonce []byte) error {
	socket := []byte(fmt.Sprintf("!<socket >%d s %s", port, nonceText(nonce)))
	marker, err := json.Marshal(ownerMarker{1, application, os.Getpid(), hex.EncodeToString(nonce)})
	if err != nil {
		return err
	}
	if err := atomicWrite(c.path, socket); err != nil {
		return fmt.Errorf("write socket description: %w", err)
	}
	if err := atomicWrite(c.path+".owner", marker); err != nil {
		_ = os.Remove(c.path)
		return fmt.Errorf("write owner marker: %w", err)
	}
	socketPath, err := windows.UTF16PtrFromString(c.path)
	if err != nil {
		_ = c.removeFiles()
		return fmt.Errorf("encode socket path: %w", err)
	}
	if err := windows.SetFileAttributes(
		socketPath,
		windows.FILE_ATTRIBUTE_SYSTEM|windows.FILE_ATTRIBUTE_READONLY,
	); err != nil {
		_ = c.removeFiles()
		return fmt.Errorf("mark Cygwin socket description: %w", err)
	}
	return nil
}

func atomicWrite(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func (c *Component) prepareExisting() error {
	_, socketErr := os.Stat(c.path)
	_, ownerErr := os.Stat(c.path + ".owner")
	if errors.Is(socketErr, os.ErrNotExist) && errors.Is(ownerErr, os.ErrNotExist) {
		return nil
	}
	if socketErr != nil || ownerErr != nil {
		return fmt.Errorf("Cygwin socket conflict: socket and owner marker must both exist")
	}
	socketInfo, err := os.Stat(c.path)
	if err != nil || !socketInfo.Mode().IsRegular() {
		return fmt.Errorf("Cygwin socket conflict: path is not a regular file")
	}
	socketData, err := os.ReadFile(c.path)
	if err != nil {
		return fmt.Errorf("read existing socket: %w", err)
	}
	markerData, err := os.ReadFile(c.path + ".owner")
	if err != nil {
		return fmt.Errorf("read owner marker: %w", err)
	}
	var marker ownerMarker
	if json.Unmarshal(markerData, &marker) != nil || marker.Version != 1 || marker.Application != application {
		return fmt.Errorf("Cygwin socket conflict: untrusted owner marker")
	}
	nonce, ok := parseSocketNonce(string(socketData))
	if !ok || marker.Nonce != hex.EncodeToString(nonce) {
		return fmt.Errorf("Cygwin socket conflict: nonce mismatch")
	}
	if processAlive(marker.PID) {
		return fmt.Errorf("Cygwin socket conflict: owner process %d is running", marker.PID)
	}
	return c.removeFiles()
}

func parseSocketNonce(value string) ([]byte, bool) {
	fields := strings.Fields(value)
	if len(fields) != 4 || fields[0] != "!<socket" || fields[2] != "s" {
		return nil, false
	}
	if _, err := strconv.Atoi(strings.TrimPrefix(fields[1], ">")); err != nil {
		return nil, false
	}
	words := strings.Split(fields[3], "-")
	if len(words) != 4 {
		return nil, false
	}
	out := make([]byte, 0, 16)
	for _, word := range words {
		b, err := hex.DecodeString(word)
		if err != nil || len(b) != 4 {
			return nil, false
		}
		out = append(out, b[3], b[2], b[1], b[0])
	}
	return out, true
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		// ERROR_INVALID_PARAMETER is returned for a PID that no longer exists.
		// Any other error (notably access denied) cannot prove staleness, so
		// preserve the files.
		return !errors.Is(err, windows.ERROR_INVALID_PARAMETER)
	}
	defer windows.CloseHandle(h)
	return true
}

func (c *Component) cleanup(nonce []byte) {
	data, err := os.ReadFile(c.path + ".owner")
	if err != nil {
		return
	}
	var marker ownerMarker
	if json.Unmarshal(data, &marker) != nil || marker.Application != application || marker.PID != os.Getpid() || marker.Nonce != hex.EncodeToString(nonce) {
		return
	}
	_ = c.removeFiles()
}

func (c *Component) removeFiles() error {
	for _, path := range []string{c.path, c.path + ".owner"} {
		ptr, err := windows.UTF16PtrFromString(path)
		if err == nil {
			_ = windows.SetFileAttributes(ptr, windows.FILE_ATTRIBUTE_NORMAL)
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
