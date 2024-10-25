package listener

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/pixil98/go-log/log"
)

type TelnetListener struct {
	port uint16

	wg sync.WaitGroup
}

func NewTelnetListener(port uint16) *TelnetListener {
	return &TelnetListener{
		port: port,
		wg:   sync.WaitGroup{},
	}
}

func (l *TelnetListener) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", l.port))
	if err != nil {
		return fmt.Errorf("listening on port %d: %w", l.port, err)
	}
	defer listener.Close()

	logger := log.GetLogger(ctx)
	logger.Infof("listening for telnet on port %d", l.port)

	for {
		select {
		case <-ctx.Done():
			l.wg.Wait()
			return nil
		default:
			conn, err := listener.Accept()
			if err != nil {
				logger.Errorf("accepting connection: %v", err)
				continue
			}

			l.wg.Add(1)
			go func() {
				l.handleConnection(ctx, conn)
				l.wg.Done()
			}()
		}
	}
}

func (l *TelnetListener) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	logger := log.GetLogger(ctx)
	logger.Infof("accepted connection from %s", conn.RemoteAddr())

	_, err := conn.Write([]byte("Hello, welcome to the mud!\n"))
	if err != nil {
		logger.Errorf("writing to connection: %v", err)
		return
	}
}
