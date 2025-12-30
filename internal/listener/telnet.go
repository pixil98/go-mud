package listener

import (
	"context"
	"fmt"
	"io"
	"sync"

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
	handler := &telnetHandler{
		cFunc:  l.cm.AcceptConnection,
		logger: log.GetLogger(ctx),
	}

	svr := telnet.NewServer(fmt.Sprintf(":%d", l.port), handler)

	// if the context is canceled, shutdown the server and all connections
	go func() {
		<-ctx.Done()
		svr.Stop()
	}()

	err := svr.ListenAndServe()
	if err != nil {
		return fmt.Errorf("serving telnet: %w", err)
	}

	return nil
}

type telnetHandler struct {
	wg     sync.WaitGroup
	cFunc  func(context.Context, io.ReadWriter)
	logger logrus.FieldLogger
	stop   chan bool
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

	ctx, cancel := context.WithCancel(context.Background())
	ctx = log.SetLogger(ctx, h.logger)

	go func() {
		<-h.stop
		cancel()
	}()

	h.cFunc(ctx, conn)
}

func (h *telnetHandler) Stop() {
	h.stop <- true
	h.wg.Wait()
}
