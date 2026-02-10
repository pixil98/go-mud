package player

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/pixil98/go-mud/internal/commands"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

type Player struct {
	conn       io.ReadWriter
	charId     storage.Identifier
	world      *game.WorldState
	cmdHandler *commands.Handler

	msgs chan []byte
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

	// Ensure subscriptions are cleaned up on exit
	defer p.world.GetPlayer(p.charId).UnsubscribeAll()

	// Show the player their current room on login
	err := p.cmdHandler.Exec(ctx, p.world, p.charId, "look")
	if err != nil {
		var userErr *commands.UserError
		if errors.As(err, &userErr) {
			_ = p.writeLine(userErr.Message)
		} else {
			return fmt.Errorf("initial look failed: %w", err)
		}
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

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
				// Input channel closed, check for scanner error
				select {
				case err := <-inputErrChan:
					return err
				default:
					return nil
				}
			}

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
	_, err := p.conn.Write([]byte("> "))
	return err
}

func (p *Player) writeLine(msg string) error {
	_, err := p.conn.Write([]byte(msg + "\n"))
	return err
}
