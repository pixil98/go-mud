package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/pixil98/go-log/log"
)

type NatsServer struct {
	ns *server.Server

	startupTimeout time.Duration
	host           string
	port           int
}

// TODO add options
func NewNatsServer(opts ...NatsServerOpt) (*NatsServer, error) {
	s := &NatsServer{
		startupTimeout: 10 * time.Second,
		host:           "127.0.0.1",
	}

	for _, opt := range opts {
		opt(s)
	}

	ns, err := server.NewServer(&server.Options{
		Host: s.host,
		Port: s.port,
	})

	s.ns = ns

	if err != nil {
		return nil, err
	}

	return s, nil
}

func (n *NatsServer) Start(ctx context.Context) error {

	n.ns.Start()

	if !n.ns.ReadyForConnections(n.startupTimeout) {
		return fmt.Errorf("nats server not ready for connections")
	}

	log.GetLogger(ctx).Infof("nats server listening on %s", n.ns.Addr())

	<-ctx.Done()
	n.ns.Shutdown()
	n.ns.WaitForShutdown()

	return nil
}
