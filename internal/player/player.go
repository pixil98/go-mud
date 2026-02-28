package player

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/pixil98/go-mud/internal/commands"
	"github.com/pixil98/go-mud/internal/game"
)

type Player struct {
	conn       io.ReadWriter
	charId     string
	world      *game.WorldState
	cmdHandler *commands.Handler

	msgs chan []byte
	done <-chan struct{}
}

// Id returns the player's unique identifier (lowercase character name)
func (p *Player) Id() string {
	return string(p.charId)
}

func (p *Player) Play(ctx context.Context) error {
	// Start goroutine to read input lines into a channel
	inputChan := make(chan string)
	inputErrChan := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(p.conn)
		for scanner.Scan() {
			inputChan <- scanner.Text()
		}
		inputErrChan <- scanner.Err()
		close(inputChan)
	}()

	// Show the player their current room on login
	err := p.cmdHandler.Exec(ctx, p.world, p.charId, "look")
	if err != nil {
		var userErr *commands.UserError
		if errors.As(err, &userErr) {
			if writeErr := p.writeLine(userErr.Message); writeErr != nil {
				slog.Warn("failed to write user error to player", "charId", p.charId, "error", writeErr)
			}
		} else {
			return fmt.Errorf("initial look failed: %w", err)
		}
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-p.done:
			ps := p.world.GetPlayer(p.charId)
			var msg string
			if ps != nil && ps.Linkless {
				msg = "\nDisconnected for inactivity."
			} else {
				msg = "\nAnother connection has taken over your session."
			}
			if err := p.writeLine(msg); err != nil {
				slog.Warn("failed to write disconnect message to player", "charId", p.charId, "error", err)
			}
			return game.ErrPlayerReconnected

		case msg := <-p.msgs:
			// Display NATS message to the player
			err = p.writeLine("\n" + string(msg))
			if err != nil {
				return err
			}
			err = p.prompt()
			if err != nil {
				return err
			}

		case line, ok := <-inputChan:
			if !ok {
				// Input channel closed (connection lost).
				// Don't clean up subscriptions — caller decides (linkless vs quit).
				select {
				case err := <-inputErrChan:
					return err
				default:
					return nil
				}
			}

			// Any input resets the idle timer.
			p.world.MarkPlayerActive(p.charId)

			line = strings.TrimSpace(line)
			if line == "" {
				err = p.prompt()
				if err != nil {
					return err
				}
				continue
			}

			// Parse command and arguments
			parts := strings.Fields(line)
			cmdName := parts[0]
			var args []string
			if len(parts) > 1 {
				args = parts[1:]
			}

			// Execute the command
			err = p.cmdHandler.Exec(ctx, p.world, p.charId, cmdName, args...)
			if err != nil {
				var userErr *commands.UserError
				if errors.As(err, &userErr) {
					err = p.writeLine(userErr.Message)
					if err != nil {
						return err
					}
				} else {
					// System error - log and disconnect
					return fmt.Errorf("command execution failed: %w", err)
				}
			}

			// Check if player wants to quit
			state := p.world.GetPlayer(p.charId)
			if state == nil {
				return fmt.Errorf("player state not found for %s", p.charId)
			}
			if state.Quit {
				p.writeLine("Goodbye!")
				// Quit handler already saved and unsubscribed isn't needed
				// — HandleSessionEnd will remove the player from the world.
				return nil
			}

			err = p.prompt()
			if err != nil {
				return err
			}
		}
	}
}

func (p *Player) prompt() error {
	prompt := "> "
	if ps := p.world.GetPlayer(p.charId); ps != nil {
		prompt = fmt.Sprintf("[%d/%dHP] > ", ps.Character.Get().CurrentHP, ps.Character.Get().MaxHP)
	}
	_, err := p.conn.Write([]byte(prompt))
	return err
}

func (p *Player) writeLine(msg string) error {
	_, err := p.conn.Write([]byte(msg + "\n\n"))
	return err
}
