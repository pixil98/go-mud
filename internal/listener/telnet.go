package listener

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"syscall"

	"github.com/iammegalith/telnet"
	"github.com/pixil98/go-log/log"
	"github.com/sirupsen/logrus"
)

type TelnetListener struct {
	port uint16
	cm   *ConnectionManager
}

func NewTelnetListener(port uint16, cm *ConnectionManager) *TelnetListener {
	return &TelnetListener{
		port: port,
		cm:   cm,
	}
}

func (l *TelnetListener) Start(ctx context.Context) error {
	// Create a cancelable context for all connections
	connCtx, cancelConns := context.WithCancel(context.Background())

	handler := &telnetHandler{
		cFunc:       l.cm.AcceptConnection,
		logger:      log.GetLogger(ctx),
		connCtx:     connCtx,
		cancelConns: cancelConns,
	}

	svr := telnet.NewServer(fmt.Sprintf(":%d", l.port), handler)

	// done signals that Start is returning (either success or failure)
	done := make(chan struct{})
	defer close(done)

	// When parent context is canceled, stop accepting and cancel all connections
	go func() {
		select {
		case <-ctx.Done():
			// Shutdown requested - stop server and handler
			svr.Stop()
			handler.Stop()
		case <-done:
			// Start returned (likely with error) - nothing to stop
		}
	}()

	err := svr.ListenAndServe()
	if err != nil {
		if errors.Is(err, syscall.EADDRINUSE) {
			return fmt.Errorf("port %d is already in use (another server running?)", l.port)
		}
		return fmt.Errorf("serving telnet on port %d: %w", l.port, err)
	}

	return nil
}

type telnetHandler struct {
	wg          sync.WaitGroup
	cFunc       func(context.Context, io.ReadWriter)
	logger      logrus.FieldLogger
	connCtx     context.Context
	cancelConns context.CancelFunc
}

func (h *telnetHandler) HandleTelnet(conn *telnet.Connection) {
	h.wg.Add(1)
	defer h.wg.Done()
	defer func() {
		err := conn.Close()
		if err != nil {
			h.logger.Errorf("closing telnet connection: %s", err)
		}
	}()

	// Use the shared context so all connections are canceled together
	ctx := log.SetLogger(h.connCtx, h.logger)

	h.cFunc(ctx, conn)
}

func (h *telnetHandler) Stop() {
	h.cancelConns()
	h.wg.Wait()
}
