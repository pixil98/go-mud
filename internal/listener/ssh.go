package listener

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/pixil98/go-log/log"
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
	defer listener.Close()

	logger := log.GetLogger(ctx)
	logger.Infof("listening for ssh on port %d", l.port)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			/*
				c, err := listener.Accept()
				if err != nil {
					logger.Errorf("accepting connection: %v", err)
					continue
				}

				conn, chans, req, err := ssh.NewServerConn(c, &ssh.ServerConfig{
					NoClientAuth: true,
				})
				if err != nil {
					logger.Errorf("handshaking: %v", err)
					continue
				}

				go ssh.DiscardRequests(req)
			*/
		}
	}
}

func (l *SshListener) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	logger := log.GetLogger(ctx)
	logger.Infof("accepted connection from %s", conn.RemoteAddr())

	_, err := conn.Write([]byte("Hello, welcome to the mud!\n"))
	if err != nil {
		logger.Errorf("writing to connection: %v", err)
		return
	}
}
