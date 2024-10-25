package command

import (
	"fmt"

	"github.com/pixil98/go-errors/errors"
	"github.com/pixil98/go-mud/internal/listener"
	"github.com/pixil98/go-service/service"
)

type ListenerType int

const (
	ListenerTypeTelnet ListenerType = iota
	ListenerTypeSSH
)

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

type ConfigListener struct {
	Protocol ListenerType `json:"protocol"`
	Port     uint16       `json:"port"`
}

func (cl *ConfigListener) Validate() error {
	el := errors.NewErrorList()

	if cl.Port == 0 {
		el.Add(fmt.Errorf("port must be set to a positive integer"))
	}

	return el.Err()
}

func (cl *ConfigListener) NewListener() (service.Worker, error) {
	switch cl.Protocol {
	case ListenerTypeTelnet:
		return listener.NewTelnetListener(cl.Port), nil
	default:
		return nil, fmt.Errorf("unknown listener type: %v", cl.Protocol)
	}
}
