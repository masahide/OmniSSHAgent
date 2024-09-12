package main

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/binary"
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

//go:embed pwsh.ps1
var pwshScript string
var debug bool

const (
	HeaderSize = 12 // 4 bytes for channel ID, 4 bytes for message length

	PacketTypeConnectSend = uint32(0)
	PacketTypeSend        = uint32(1)
	PacketTypeClose       = uint32(2)
)

type Packet struct {
	PacketType uint32
	ChannelID  uint32
	Payload    []byte
}

type Multiplexer struct {
	writer     io.Writer
	reader     *bufio.Reader
	channels   map[uint32]chan []byte
	channelsMu sync.Mutex
}

func NewMultiplexer(writer io.Writer, reader io.Reader) *Multiplexer {
	mux := &Multiplexer{
		writer:   writer,
		reader:   bufio.NewReader(reader),
		channels: make(map[uint32]chan []byte),
	}
	//go mux.readLoop()
	return mux
}

func (mux *Multiplexer) readLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		header := make([]byte, HeaderSize)
		_, err := io.ReadFull(mux.reader, header)
		if err != nil {
			if errors.Is(err, io.EOF) {
				if debug {
					log.Println("Connection closed")
				}
				return err
			}
			log.Println("Error reading header:", err)
			return err
		}
		if debug {
			log.Printf("mux readFull header:[%v]", header)
		}

		packetType := binary.LittleEndian.Uint32(header[:4])
		channelID := binary.LittleEndian.Uint32(header[4:])
		length := binary.LittleEndian.Uint32(header[8:])
		payload := make([]byte, length)
		_, err = io.ReadFull(mux.reader, payload)
		if err != nil {
			log.Println("Error reading payload:", err)
			return err
		}
		if debug {
			log.Printf("mux readFull payload type:%d, ch:%d, len:%d ", packetType, channelID, length)
		}

		switch packetType {
		case PacketTypeSend:
			mux.channelsMu.Lock()
			ch, ok := mux.channels[channelID]
			mux.channelsMu.Unlock()
			if ok {
				ch <- payload
			} else {
				log.Printf("mux readFull error: channel %d not found", channelID)
			}
		case PacketTypeClose:
			mux.CloseChannel(channelID)
			if debug {
				log.Printf("mux readFull close channel %d", channelID)
			}
		}
	}
}

func (mux *Multiplexer) WriteChannel(packet Packet) error {
	buf := make([]byte, HeaderSize+len(packet.Payload))
	binary.LittleEndian.PutUint32(buf[0:4], packet.PacketType)
	binary.LittleEndian.PutUint32(buf[4:8], packet.ChannelID)
	binary.LittleEndian.PutUint32(buf[8:HeaderSize], uint32(len(packet.Payload)))
	copy(buf[HeaderSize:], packet.Payload)
	_, err := mux.writer.Write(buf)
	//n, err := mux.writer.Write(buf)
	//log.Printf("type:%d, ch:%d, len:%d,buf:[%v] n:%d", packet.PacketType, packet.ChannelID, len(packet.Payload), buf, n)
	return err
}

func (mux *Multiplexer) OpenChannel(channelID uint32) chan []byte {
	mux.channelsMu.Lock()
	defer mux.channelsMu.Unlock()

	ch := make(chan []byte, 10)
	mux.channels[channelID] = ch
	return ch
}

func (mux *Multiplexer) CloseChannel(channelID uint32) {
	mux.channelsMu.Lock()
	defer mux.channelsMu.Unlock()

	if ch, ok := mux.channels[channelID]; ok {
		close(ch)
		delete(mux.channels, channelID)
	}
}

func (ps *pwshIOStream) handleConnection(ctx context.Context, conn net.Conn, channelID uint32) {
	defer conn.Close()
	ch := ps.OpenChannel(channelID)
	go func() {
		defer func() {
			ps.WriteChannel(Packet{PacketType: PacketTypeClose, ChannelID: channelID, Payload: []byte{}})
			ps.CloseChannel(channelID)
		}()
		packetType := PacketTypeConnectSend
		for {
			payload := make([]byte, 4096)
			n, err := conn.Read(payload)
			if err != nil {
				if err == io.EOF {
					if debug {
						log.Printf("DomainSocket.read ch:%d io.EOF", channelID)
					}
					break
				}
				log.Println("Error reading from connection:", err)
				break
			}
			ps.WriteChannel(Packet{PacketType: packetType, ChannelID: channelID, Payload: payload[:n]})
			packetType = PacketTypeSend
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	domainSocketWriter := bufio.NewWriter(conn)
	for msg := range ch {
		_, err := domainSocketWriter.Write(msg)
		if err != nil {
			log.Println("Error writing to connection:", err)
			break
		}
		domainSocketWriter.Flush()
		if debug {
			log.Printf("DomainSocketWriter.Write ch:%d len:%d msg:%v", channelID, len(msg), msg)
		}
	}
	if debug {
		log.Printf("Close DomainSocket ch:%d", channelID)
	}
}

type pwshIOStream struct {
	*Multiplexer
	exePath string

	cmd    *exec.Cmd
	out    io.ReadCloser
	in     io.WriteCloser
	cancel context.CancelFunc
}

func (ps *pwshIOStream) setCancel(cancel context.CancelFunc) {
	ps.cancel = cancel
}

func NewPwshIOStream(exePath string) *pwshIOStream {
	return &pwshIOStream{
		exePath: exePath,
	}
}

/*
func (ps *pwshIOStream) readLoop() error {
	return ps.Multiplexer.readLoop()
}
*/

func (ps *pwshIOStream) sigStopWorker() {
	// Capture kill signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	log.Printf("Received signal: %s. Shutting down PowerShell process...", sig)
	ps.killPwsh()
	os.Exit(0)
}

func (ps *pwshIOStream) startPowerShellProces(ctx context.Context) {
	log.Println("start PowerShell process...")
	ps.cmd = exec.Command(ps.exePath, "-NoProfile", "-Command", "-")

	defer ps.cancel()
	var err error
	ps.in, err = ps.cmd.StdinPipe()
	if err != nil {
		log.Printf("cmd.StdinPipe() err:%s", err)
		return
	}
	ps.out, err = ps.cmd.StdoutPipe()
	if err != nil {
		log.Printf("cmd.StdoutPipe() err:%s", err)
		return
	}
	ps.cmd.Stderr = os.Stderr

	if err := ps.cmd.Start(); err != nil {
		log.Printf("cmd.Start() err:%s", err)
		return
	}
	log.Printf("Started PowerShell process with PID: %d", ps.cmd.Process.Pid)

	if debug {
		pwshScript = uncommentWriteLines(pwshScript)
	}
	io.WriteString(ps.in, pwshScript)
	ps.Multiplexer = NewMultiplexer(ps.in, ps.out)
	checkStartAgent(ps.out)
	if err = ps.readLoop(ctx); err != nil {
		log.Printf("readLoop() err:%s", err)
		return
	}
}

func (ps *pwshIOStream) killPwsh() {
	if err := ps.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("send signal err:%s", err)
	}
	if ps.in != nil {
		ps.in.Close()
	}
	if ps.out != nil {
		ps.out.Close()
	}
	log.Printf("PowerShell process exited")
}

func checkStartAgent(r io.Reader) {
	buf := make([]byte, 1024)
	n, err := r.Read(buf)
	if err != nil {
		log.Printf("read err: %s", err)
		return
	}
	if !bytes.Equal(buf[:n], []byte("startAgent")) {
		log.Printf(" startAgent err:?:[%s]", string(buf[:n]))
		os.Exit(1)
	}
	log.Print("start powershell")
}

func getSystem32Path() string {
	path := os.Getenv("PATH")
	paths := strings.Split(path, ":")
	for _, p := range paths {
		if strings.HasPrefix(p, "/mnt/") && strings.HasSuffix(p, "/Windows/system32") {
			return p
		}
	}
	return ""
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.BoolVar(&debug, "debug", false, "debug mode")
	flag.Parse()
	exePath := getSystem32Path()
	if len(exePath) > 0 {
		exePath = filepath.Join(exePath, "WindowsPowerShell/v1.0/powershell.exe")
	}
	_, err := os.Stat(exePath)
	if err != nil {
		exePath = ("powershell.exe")
	}
	socketPath := os.Getenv("SSH_AUTH_SOCK")
	if len(socketPath) == 0 {
		log.Fatal("env SSH_AUTH_SOCK is not set")
	}
	log.Printf("get env SSH_AUTH_SOCK:%s", socketPath)
	if _, err := os.Stat(socketPath); err == nil {
		err := os.Remove(socketPath)
		if err != nil {
			log.Fatal("remove old socket err:", err)
		}
	}

	ps := NewPwshIOStream(exePath)
	defer ps.killPwsh()
	go ps.sigStopWorker()
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Println("Error creating Unix domain socket:", err)
		return
	}
	defer listener.Close()
	for {
		// Start PowerShell process
		ctx, cancel := context.WithCancel(context.Background())
		ps.setCancel(cancel)
		go ps.startPowerShellProces(ctx)
		log.Printf("listen socket:%s", socketPath)
		ps.listenLoop(ctx, listener)
		cancel()
	}
}

func (ps *pwshIOStream) listenLoop(ctx context.Context, listener net.Listener) error {
	var channelID uint32 = 1
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				log.Println("Listener has been closed.")
				return err
			}
			log.Println("Error accepting connection:", err)
			continue
		}
		if debug {
			log.Printf("domainSocket:%v accept ch:%d", conn.LocalAddr(), channelID)
		}
		go ps.handleConnection(ctx, conn, channelID)
		channelID++
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
}

func uncommentWriteLines(script string) string {
	lines := strings.Split(script, "\n")
	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "# [Console]::Error.WriteLine(") {
			lines[i] = strings.Replace(line, "# ", "", 1)
		}
	}
	return strings.Join(lines, "\n")
}
