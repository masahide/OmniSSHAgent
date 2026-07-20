//go:build windows && e2e

package e2e

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func TestLiveCygwinInterfaceListsAndSigns(t *testing.T) {
	socketPath := os.Getenv("OMNISSHAGENT_CYGWIN_SOCKET")
	if socketPath == "" {
		t.Skip("OMNISSHAGENT_CYGWIN_SOCKET is not set")
	}
	description, err := os.ReadFile(socketPath)
	if err != nil {
		t.Fatal(err)
	}
	port, nonce, err := parseDescription(string(description))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialTimeout("tcp4", fmt.Sprintf("127.0.0.1:%d", port), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write(nonce); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 16)
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(reply, nonce) {
		t.Fatal("nonce response does not match")
	}
	if _, err := conn.Write(make([]byte, 12)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(conn, make([]byte, 12)); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetDeadline(time.Time{})

	client := agent.NewClient(conn)
	keys, err := client.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) == 0 {
		t.Fatal("OpenSSH Agent returned no keys")
	}
	publicKey, err := ssh.ParsePublicKey(keys[0].Blob)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := client.Sign(publicKey, []byte("OmniSSHAgent live Cygwin E2E"))
	if err != nil {
		t.Fatal(err)
	}
	if signature == nil || len(signature.Blob) == 0 {
		t.Fatal("OpenSSH Agent returned an empty signature")
	}
}

func parseDescription(value string) (int, []byte, error) {
	fields := strings.Fields(value)
	if len(fields) != 4 || fields[0] != "!<socket" || fields[2] != "s" {
		return 0, nil, fmt.Errorf("invalid Cygwin socket description")
	}
	port, err := strconv.Atoi(strings.TrimPrefix(fields[1], ">"))
	if err != nil {
		return 0, nil, fmt.Errorf("invalid port: %w", err)
	}
	words := strings.Split(fields[3], "-")
	if len(words) != 4 {
		return 0, nil, fmt.Errorf("invalid nonce")
	}
	nonce := make([]byte, 0, 16)
	for _, word := range words {
		encoded, err := hex.DecodeString(word)
		if err != nil || len(encoded) != 4 {
			return 0, nil, fmt.Errorf("invalid nonce word")
		}
		nonce = append(nonce, encoded[3], encoded[2], encoded[1], encoded[0])
	}
	return port, nonce, nil
}
