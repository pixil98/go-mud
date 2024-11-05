package nats

import "time"

type NatsServerOpt func(*NatsServer)

// WithStartTimeout sets the startup timeout for the nats server
func WithStartTimeout(d time.Duration) NatsServerOpt {
	return func(n *NatsServer) {
		n.startupTimeout = d
	}
}

// WithHost sets the host for the nats server
func WithHost(host string) NatsServerOpt {
	return func(n *NatsServer) {
		n.host = host
	}
}

// WithPort sets the port for the nats server
func WithPort(port int) NatsServerOpt {
	return func(n *NatsServer) {
		n.port = port
	}
}
