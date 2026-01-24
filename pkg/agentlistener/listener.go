package agentlistener

import (
	"context"
	"io"
	"log"
	"net"
)

// Serve runs Accept loop for listener and closes it when ctx is cancelled.
func Serve(ctx context.Context, listener net.Listener, handler func(context.Context, net.Conn)) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctxCloser := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			listener.Close()
		case <-ctxCloser:
		}
	}()
	defer close(ctxCloser)

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				log.Println("agentlistener: listener closed due to context cancellation")
				return nil
			}
			if err == io.EOF {
				return nil
			}
			log.Printf("agentlistener: Accept error: %v", err)
			return err
		}
		go handler(ctx, conn)
	}
}
