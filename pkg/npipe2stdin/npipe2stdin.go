//go:build windows
// +build windows

package npipe2stdin

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/Microsoft/go-winio"
)

type Npipe2Stdin struct {
	Name  string
	Debug bool
}

func pipe(out io.Writer, in io.Reader) chan error {
	errCh := make(chan error, 1)
	go func() {
		_, err := io.Copy(out, in)
		errCh <- err
		close(errCh)
	}()
	return errCh
}

func getFirstErr(ctx context.Context, a, b chan error) error {
	var err error
	select {
	case err = <-a:
	case err = <-b:
	case <-ctx.Done():
	}
	return err
}

func (n *Npipe2Stdin) Proxy(ctx context.Context) error {
	pipePath := map[bool]string{
		false: `\\.\pipe\` + n.Name,
		true:  `\\.\pipe\openssh-ssh-agent`,
	}[len(n.Name) == 0]
	if n.Debug {
		log.Print("Started omni-socat")
	}
	conn, err := winio.DialPipe(pipePath, nil)
	if err != nil {
		return fmt.Errorf("winio.DialPipe[%s] err:%w", pipePath, err)
	}
	if n.Debug {
		log.Printf("Opened pipe:[%s]", pipePath)
	}
	defer conn.Close()
	outErrCh := pipe(os.Stdout, conn)
	inErrCh := pipe(conn, os.Stdin)
	err = getFirstErr(ctx, outErrCh, inErrCh)
	if n.Debug {
		log.Printf("Disconnected. [%v]", err)
	}
	if err != nil && err != io.EOF {
		return fmt.Errorf("pipe err:%w", err)
	}
	return nil
}
