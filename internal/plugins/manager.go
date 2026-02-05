package plugins

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/pixil98/go-mud/internal/game"
)

// InfoStyle indicates the display context for character info
type InfoStyle int

const (
	InfoStyleShort InfoStyle = iota // brief display (e.g., "who" command)
	InfoStyleFull                   // detailed display (e.g., "score" command)
)

// CharacterInitializer handles character initialization
type CharacterInitializer interface {
	InitCharacter(rw io.ReadWriter, char *game.Character) error
}

// CharacterInfoProvider returns display info about characters
// TODO: Add importance/priority levels to returned info so callers can:
//   - Order items consistently in displays like "score"
//   - Filter to only show important items in compact displays like "who"
type CharacterInfoProvider interface {
	GetCharacterInfo(char *game.Character, style InfoStyle) map[string]string
}

// PluginServices combines all plugin service interfaces.
// Use this when you need access to multiple plugin capabilities.
type PluginServices interface {
	CharacterInitializer
	CharacterInfoProvider
}

// Plugin defines the full interface that plugin implementations must satisfy.
type Plugin interface {
	game.TickHandler
	PluginServices
	Key() string
	Init() error
}

type PluginManager struct {
	plugins []Plugin
}

func NewPluginManager() *PluginManager {
	return &PluginManager{plugins: []Plugin{}}
}

func (m *PluginManager) Register(p Plugin) error {
	if p == nil {
		return fmt.Errorf("plugin is nil")
	}

	m.plugins = append(m.plugins, p)
	slog.Info("registered plugin", "key", p.Key())

	return p.Init()
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

func (m *PluginManager) InitCharacter(rw io.ReadWriter, char *game.Character) error {
	for _, p := range m.plugins {
		err := p.InitCharacter(rw, char)
		if err != nil {
			return fmt.Errorf("initCharacter plugin %s: %w ", p.Key(), err)
		}
	}

	return nil
}

// GetCharacterInfo aggregates character info from all plugins
func (m *PluginManager) GetCharacterInfo(char *game.Character, style InfoStyle) map[string]string {
	result := make(map[string]string)
	for _, p := range m.plugins {
		for k, v := range p.GetCharacterInfo(char, style) {
			result[k] = v
		}
	}
	return result
}
