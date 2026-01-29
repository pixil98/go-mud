package plugins

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/pixil98/go-mud/internal/game"
)

type Extensible interface {
	Set(string, any) error
	Get(string, any) (bool, error)
}

type Plugin interface {
	game.TickHandler
	Key() string
	Init(context.Context) error
	OnInitCharacter(io.ReadWriter, Extensible) error
}

type PluginManager struct {
	plugins []Plugin
}

func NewPluginManager() *PluginManager {
	return &PluginManager{plugins: []Plugin{}}
}

func (m *PluginManager) Register(ctx context.Context, p Plugin) error {
	if p == nil {
		return fmt.Errorf("plugin is nil")
	}

	m.plugins = append(m.plugins, p)
	slog.InfoContext(ctx, "registered plugin", "key", p.Key())

	return p.Init(ctx)
}

func (m *PluginManager) Tick(ctx context.Context) error {
	for _, p := range m.plugins {
		err := p.Tick(ctx)
		if err != nil {
			return fmt.Errorf("ticking %s: %w", p.Key(), err)
		}
	}

	return nil
}

func (m *PluginManager) InitCharacter(rw io.ReadWriter, e Extensible) error {
	for _, p := range m.plugins {
		err := p.OnInitCharacter(rw, e)
		if err != nil {
			return fmt.Errorf("initCharacter plugin %s: %w ", p.Key(), err)
		}
	}

	return nil
}
