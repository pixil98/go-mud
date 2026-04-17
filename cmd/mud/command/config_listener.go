package command

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/pixil98/go-mud/internal/listener"
	"github.com/pixil98/go-service"
	"golang.org/x/crypto/ssh"
)

// ListenerType identifies the network protocol used by a listener.
type ListenerType int

// ListenerType values.
const (
	ListenerTypeTelnet ListenerType = iota
	ListenerTypeSSH
)

// UnmarshalText decodes a JSON/YAML string into a ListenerType.
func (lt *ListenerType) UnmarshalText(text []byte) error {
	switch string(text) {
	case "telnet":
		*lt = ListenerTypeTelnet
	case "ssh":
		*lt = ListenerTypeSSH
	default:
		return fmt.Errorf("unknown listener type: %s", text)
	}
	return nil
}

// ListenerConfig holds the configuration for a single network listener.
type ListenerConfig struct {
	Protocol    ListenerType `json:"protocol"`
	Port        uint16       `json:"port"`
	HostKeyPath string       `json:"host_key_path,omitempty"`
}

func (cl *ListenerConfig) validate() error {
	var errs []error

	if cl.Port == 0 {
		errs = append(errs, fmt.Errorf("port must be set to a positive integer"))
	}

	return errors.Join(errs...)
}

// BuildListener creates the appropriate service.Worker for this listener's protocol.
func (cl *ListenerConfig) BuildListener(cm *listener.ConnectionManager) (service.Worker, error) {
	switch cl.Protocol {
	case ListenerTypeTelnet:
		return listener.NewTelnetListener(cl.Port, cm), nil
	case ListenerTypeSSH:
		hostKey, err := cl.loadOrGenerateHostKey()
		if err != nil {
			return nil, fmt.Errorf("setting up ssh host key: %w", err)
		}
		return listener.NewSSHListener(cl.Port, cm, hostKey), nil
	default:
		return nil, fmt.Errorf("unknown listener type: %v", cl.Protocol)
	}
}

func (cl *ListenerConfig) loadOrGenerateHostKey() (ssh.Signer, error) {
	if cl.HostKeyPath != "" {
		keyBytes, err := os.ReadFile(cl.HostKeyPath)
		if err != nil {
			return nil, fmt.Errorf("reading host key %q: %w", cl.HostKeyPath, err)
		}
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("parsing host key %q: %w", cl.HostKeyPath, err)
		}
		return signer, nil
	}

	slog.Warn("no host_key_path configured for ssh listener, generating ephemeral key")
	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating ephemeral key: %w", err)
	}
	signer, err := ssh.NewSignerFromKey(privKey)
	if err != nil {
		return nil, fmt.Errorf("creating signer from ephemeral key: %w", err)
	}
	return signer, nil
}
