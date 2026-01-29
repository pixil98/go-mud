package listener

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
)

type SshListener struct {
	port uint16

	wg sync.WaitGroup
}

func NewSshListener(port uint16) *SshListener {
	return &SshListener{
		port: port,
		wg:   sync.WaitGroup{},
	}
}

func (l *SshListener) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", l.port))
	if err != nil {
		return fmt.Errorf("listening on port %d: %w", l.port, err)
	}
	defer func() {
		err := listener.Close()
		if err != nil {
			slog.ErrorContext(ctx, "closing listener", "error", err)
		}
	}()

	slog.InfoContext(ctx, "listening for ssh", "port", l.port)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			return nil
			/*
				c, err := listener.Accept()
				if err != nil {
					slog.ErrorContext(ctx, "accepting connection", "error", err)
					continue
				}

				conn, chans, req, err := ssh.NewServerConn(c, &ssh.ServerConfig{
					NoClientAuth: true,
				})
				if err != nil {
					slog.ErrorContext(ctx, "handshaking", "error", err)
					continue
				}

				go ssh.DiscardRequests(req)
			*/
		}
	}
}

func (l *SshListener) handleConnection(ctx context.Context, conn net.Conn) {
	defer func() {
		err := conn.Close()
		if err != nil {
			slog.ErrorContext(ctx, "closing ssh connection", "error", err)
		}
	}()

	slog.InfoContext(ctx, "accepted connection", "remote_addr", conn.RemoteAddr())

	_, err := conn.Write([]byte("Hello, welcome to the mud!\n"))
	if err != nil {
		slog.ErrorContext(ctx, "writing to connection", "error", err)
		return
	}
}
