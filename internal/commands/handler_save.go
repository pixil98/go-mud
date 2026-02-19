package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// SaveHandlerFactory creates handlers that persist the player's character.
type SaveHandlerFactory struct {
	chars storage.Storer[*game.Character]
	pub   Publisher
}

func NewSaveHandlerFactory(chars storage.Storer[*game.Character], pub Publisher) *SaveHandlerFactory {
	return &SaveHandlerFactory{chars: chars, pub: pub}
}

func (f *SaveHandlerFactory) Spec() *HandlerSpec {
	return nil
}

func (f *SaveHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *SaveHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		if err := saveCharacter(f.chars, cmdCtx.Session); err != nil {
			return fmt.Errorf("saving character: %w", err)
		}

		if f.pub != nil {
			return f.pub.PublishToPlayer(cmdCtx.Session.CharId, []byte("Character saved."))
		}

		return nil
	}, nil
}

// saveCharacter persists the character's current session state (location, inventory, etc.)
// to the character store. Shared by save and quit handlers.
func saveCharacter(chars storage.Storer[*game.Character], session *game.PlayerState) error {
	zoneId, roomId := session.Location()
	session.Character.LastZone = zoneId
	session.Character.LastRoom = roomId

	return chars.Save(string(session.CharId), session.Character)
}
