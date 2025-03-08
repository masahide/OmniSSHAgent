package cygwinsocket

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/sys/windows"
)

type CygwinSock struct {
	agent.ExtendedAgent
	Debug    bool
	Path     string
	listener net.Listener
}

func New() *CygwinSock {
	return &CygwinSock{}
}

func (c *CygwinSock) RunAgent() error {
	if len(c.Path) == 0 {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("userHomeDir err:%w", err)
		}
		c.Path = filepath.Join(home, "OmniSSHcygwin.sock")
	}
	_, err := os.Stat(c.Path)
	if err == nil || !os.IsNotExist(err) {
		err = os.Remove(c.Path)
		if err != nil {
			if c.Debug {
				log.Printf("Failed remove socket %s, err:%s", c.Path, err)
			}
			return fmt.Errorf("Failed remove socket %s, err:%w", c.Path, err)
		}
	}

	c.listener, err = net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		if c.Debug {
			log.Printf("net.Listen err:%s", err)
		}
		return fmt.Errorf("net.Listen err: %w", err)
	}

	port := c.listener.Addr().(*net.TCPAddr).Port
	nonce, err := c.makeSocket(c.Path, port)
	if err != nil {
		if c.Debug {
			log.Printf("makeSocket err:%s", err)
		}
		return err
	}
	go func() {
		for {
			conn, err := c.listener.Accept()
			if err != nil {
				if c.Debug {
					log.Printf("listener.Accept err:%s", err)
				}
				return
			}
			if err := c.handle(conn, nonce); err != nil {
				log.Println(err)
			}

		}
	}()
	return nil
}

func nonce2s(buf []byte) string {
	hexstrs := make([]string, 0, 4)
	for ; len(buf) > 0; buf = buf[4:] {
		hexstrs = append(hexstrs, hex.EncodeToString([]byte{buf[3], buf[2], buf[1], buf[0]}))
	}
	return strings.Join(hexstrs, "-")
}

func (c *CygwinSock) handle(conn net.Conn, nonce []byte) error {
	nonceR := make([]byte, 16)
	if _, err := conn.Read(nonceR); err != nil {
		return err
	}
	if !bytes.Equal(nonce, nonceR) {
		return fmt.Errorf("nonce not Equal: %x:%x", nonce, nonceR)
	}
	if _, err := conn.Write(nonce); err != nil {
		return err
	}
	buf := make([]byte, 12)
	if _, err := conn.Read(buf); err != nil {
		return err
	}
	binary.LittleEndian.PutUint32(buf, uint32(os.Getpid()))
	if _, err := conn.Write(buf); err != nil {
		return err
	}
	if c.Debug {
		log.Print("Start Cygwin Unix doman socket: ServeAgent")
	}
	if err := agent.ServeAgent(c, conn); err != nil {
		if err == io.EOF {
			return nil
		}
		return fmt.Errorf("Cygwin agent.ServeAgent err:%w", err)
	}
	return nil
}

func (c *CygwinSock) makeSocket(filePath string, port int) (nonce []byte, err error) {

	gid, err := uuid.NewUUID()
	if err != nil {
		return
	}
	nonce, err = gid.MarshalBinary()
	if err != nil {
		return //err
	}

	if err = os.WriteFile(filePath,
		[]byte(fmt.Sprintf("!<socket >%d s %s", port, nonce2s(nonce))),
		0600); err != nil {
		return
	}
	var f *uint16
	if f, err = windows.UTF16PtrFromString(filePath); err != nil {
		return
	}
	err = windows.SetFileAttributes(
		f,
		windows.FILE_ATTRIBUTE_SYSTEM|windows.FILE_ATTRIBUTE_READONLY,
	)
	if c.Debug {
		log.Printf("Serving %s:%d with nonce: %s", filePath, port, nonce2s(nonce))
	}
	return
}
