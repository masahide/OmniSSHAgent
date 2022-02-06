package npipe2stdin

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/Microsoft/go-winio"
)

type Npipe2Stdin struct {
	Name string
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
		true:  `\\.\pipe\` + n.Name,
		false: `\\.\pipe\openssh-ssh-agent`,
	}[len(n.Name) == 0]
	conn, err := winio.DialPipe(pipePath, nil)
	if err != nil {
		return fmt.Errorf("winio.DialPipe err:%w", err)
	}
	defer conn.Close()
	outErrCh := pipe(os.Stdout, conn)
	inErrCh := pipe(conn, os.Stdin)
	err = getFirstErr(ctx, outErrCh, inErrCh)
	if err != nil && err != io.EOF {
		return fmt.Errorf("pipe err:%w", err)
	}
	return nil
}
