package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

type NatsServer struct {
	ns   *server.Server
	conn *nats.Conn

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
		Host:   s.host,
		Port:   s.port,
		NoSigs: true, // Let the application handle signals
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

	// Create internal client connection
	conn, err := nats.Connect(n.clientURL())
	if err != nil {
		return fmt.Errorf("creating nats client connection: %w", err)
	}
	n.conn = conn

	slog.InfoContext(ctx, "nats server listening", "addr", n.ns.Addr())

	<-ctx.Done()
	n.conn.Close()
	n.ns.Shutdown()
	n.ns.WaitForShutdown()

	return nil
}

// Subscribe creates a subscription on the given subject.
// The handler is called for each message received.
// Returns an unsubscribe function to remove the subscription.
func (n *NatsServer) Subscribe(subject string, handler func(data []byte)) (func(), error) {
	if n.conn == nil {
		return nil, fmt.Errorf("nats server not started")
	}
	sub, err := n.conn.Subscribe(subject, func(msg *nats.Msg) {
		handler(msg.Data)
	})
	if err != nil {
		return nil, err
	}
	return func() { sub.Unsubscribe() }, nil
}

// Publish sends a message to the given subject
func (n *NatsServer) Publish(subject string, data []byte) error {
	if n.conn == nil {
		return fmt.Errorf("nats server not started")
	}
	return n.conn.Publish(subject, data)
}

func (n *NatsServer) clientURL() string {
	return fmt.Sprintf("nats://%s:%d", n.host, n.port)
}
