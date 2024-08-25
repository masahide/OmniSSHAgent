package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/binary"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed pwsh.ps1
var pwshScript string

const (
	DEBUG      = false
	HeaderSize = 12 // 4 bytes for channel ID, 4 bytes for message length

	PacketTypeConnectSend = uint32(0)
	PacketTypeSend        = uint32(1)
	PacketTypeClose       = uint32(2)
)

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
	go mux.readLoop()
	return mux
}

type Packet struct {
	PacketType uint32
	ChannelID  uint32
	Payload    []byte
}

func (mux *Multiplexer) readLoop() {
	if DEBUG {
		for {
			buf := make([]byte, 4096)
			n, err := mux.reader.Read(buf)
			log.Printf("debug read buf:[%s] n:%d,err=%v", buf, n, err)
		}
	}
	for {
		header := make([]byte, HeaderSize)
		_, err := io.ReadFull(mux.reader, header)
		if err != nil {
			log.Println("Error reading header:", err)
			return
		}

		packetType := binary.LittleEndian.Uint32(header[:4])
		channelID := binary.LittleEndian.Uint32(header[4:])
		length := binary.LittleEndian.Uint32(header[8:])
		payload := make([]byte, length)
		_, err = io.ReadFull(mux.reader, payload)
		if err != nil {
			log.Println("Error reading payload:", err)
			return
		}
		//log.Printf("mux readFull payload type:%d, ch:%d, len:%d ", packetType, channelID, length)

		switch packetType {
		case PacketTypeSend:
			mux.channelsMu.Lock()
			if ch, ok := mux.channels[channelID]; ok {
				ch <- payload
			}
			mux.channelsMu.Unlock()
		case PacketTypeClose:
			mux.CloseChannel(channelID)
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

func handleConnection(conn net.Conn, mux *Multiplexer, channelID uint32) {
	defer conn.Close()
	ch := mux.OpenChannel(channelID)
	go func() {
		//reader := bufio.NewReader(conn)
		packetType := PacketTypeConnectSend
		for {
			payload := make([]byte, 4096)
			n, err := conn.Read(payload)
			if err != nil {
				if err == io.EOF {
					break
				}
				log.Println("Error reading from connection:", err)
				break
			}
			//log.Printf("handleCoonection read:  channelID:%d byte[%v]", channelID, payload[:n])
			mux.WriteChannel(Packet{PacketType: packetType, ChannelID: channelID, Payload: payload[:n]})
			packetType = PacketTypeSend
		}
		mux.WriteChannel(Packet{PacketType: PacketTypeClose, ChannelID: channelID, Payload: []byte{}})
		mux.CloseChannel(channelID)
	}()

	writer := bufio.NewWriter(conn)
	for msg := range ch {
		_, err := writer.Write(msg)
		if err != nil {
			log.Println("Error writing to connection:", err)
			break
		}
		writer.Flush()
	}
}
func checkStartAgent(r io.Reader) {
	buf := make([]byte, 1024)
	n, err := r.Read(buf)
	if err != nil {
		log.Printf("read err: %s", err)
		return
	}
	if DEBUG {
		log.Printf("debug read: %s", string(buf[:n]))
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
	exePath := getSystem32Path()
	if len(exePath) > 0 {
		exePath = filepath.Join(exePath, "WindowsPowerShell/v1.0/powershell.exe")
	}
	_, err := os.Stat(exePath)
	if err != nil {
		exePath = ("powershell.exe")
	}
	cmd := exec.Command(exePath, "-NoProfile", "-Command", "-")

	psIn, err := cmd.StdinPipe()
	if err != nil {
		log.Println("Error creating stdin pipe:", err)
		return
	}
	psOut, err := cmd.StdoutPipe()
	if err != nil {
		log.Println("Error creating stdout pipe:", err)
		return
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Println("Error starting command:", err)
		return
	}

	io.WriteString(psIn, pwshScript)

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("writeString powershellScript")

	checkStartAgent(psOut)

	mux := NewMultiplexer(psIn, psOut)

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
	log.Printf("listen socket:%s", socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Println("Error creating Unix domain socket:", err)
		return
	}
	defer listener.Close()

	var channelID uint32 = 1
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error accepting connection:", err)
			continue
		}
		//log.Printf("accept: %v", conn.LocalAddr())
		go handleConnection(conn, mux, channelID)
		channelID++
	}
}
