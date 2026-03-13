package listener

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"
)

// SSHListener accepts incoming SSH connections and hands them off to the ConnectionManager.
type SSHListener struct {
	port    uint16
	cm      *ConnectionManager
	hostKey ssh.Signer
}

// NewSSHListener creates an SSHListener on the given port using the provided host key.
func NewSSHListener(port uint16, cm *ConnectionManager, hostKey ssh.Signer) *SSHListener {
	return &SSHListener{
		port:    port,
		cm:      cm,
		hostKey: hostKey,
	}
}

// Start begins accepting SSH connections until ctx is canceled.
func (l *SSHListener) Start(ctx context.Context) error {
	config := &ssh.ServerConfig{
		NoClientAuth: true,
	}
	config.AddHostKey(l.hostKey)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", l.port))
	if err != nil {
		return fmt.Errorf("listening on port %d: %w", l.port, err)
	}

	slog.InfoContext(ctx, "listening for ssh", "port", l.port)

	connCtx, cancelConns := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	// Close the listener when the parent context is canceled
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			// Check if shutdown was requested
			select {
			case <-ctx.Done():
				cancelConns()
				wg.Wait()
				return nil
			default:
			}
			slog.ErrorContext(ctx, "accepting ssh connection", "error", err)
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			l.handleConnection(connCtx, conn, config)
		}()
	}
}

func (l *SSHListener) handleConnection(ctx context.Context, conn net.Conn, config *ssh.ServerConfig) {
	defer func() { _ = conn.Close() }()

	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		slog.ErrorContext(ctx, "ssh handshake", "remote", conn.RemoteAddr(), "error", err)
		return
	}
	defer func() { _ = sshConn.Close() }()

	slog.InfoContext(ctx, "ssh connection established", "remote", conn.RemoteAddr())

	// Close the SSH connection when the context is cancelled.
	// This unblocks the channel iteration loop below so handleConnection can return.
	go func() {
		<-ctx.Done()
		_ = sshConn.Close()
	}()

	go ssh.DiscardRequests(reqs)

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			_ = newChan.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		ch, requests, err := newChan.Accept()
		if err != nil {
			slog.ErrorContext(ctx, "accepting ssh channel", "error", err)
			continue
		}

		// Wait for the client to request a shell before starting the session.
		// SSH clients won't forward input until they receive the shell reply.
		shellReady := make(chan struct{})
		go func(in <-chan *ssh.Request) {
			for req := range in {
				switch req.Type {
				case "pty-req":
					// Reject PTY so the client keeps local echo and line buffering.
					_ = req.Reply(false, nil)
				case "shell":
					_ = req.Reply(true, nil)
					close(shellReady)
				default:
					_ = req.Reply(false, nil)
				}
			}
		}(requests)

		select {
		case <-shellReady:
		case <-ctx.Done():
			_ = ch.Close()
			continue
		}

		l.cm.AcceptConnection(ctx, newCRLFReadWriter(ch))
		_ = ch.Close()
	}
}
