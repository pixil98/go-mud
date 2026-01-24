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
)

type Player struct {
	conn       io.ReadWriter
	char       *Character
	cmdHandler *commands.Handler
	game.EntityState

	// Subscriber for creating new subscriptions
	subscriber Subscriber
	// Cleanup functions for active subscriptions (player, room, group, etc.)
	subs map[string]func()
	msgs chan []byte
}

func (p *Player) Tick(ctx context.Context) {
	// do something like regen here
	//p.character.Regen()
}

// Unsubscribe removes a subscription by name
func (p *Player) Unsubscribe(name string) {
	if unsub, ok := p.subs[name]; ok {
		unsub()
		delete(p.subs, name)
	}
}

// Subscribe creates a new subscription for the player
func (p *Player) Subscribe(name, subject string) error {
	if p.subscriber == nil {
		return nil
	}
	unsub, err := p.subscriber.Subscribe(subject, func(data []byte) {
		p.msgs <- data
	})
	if err != nil {
		return err
	}
	p.subs[name] = unsub
	return nil
}

// UnsubscribeAll removes all subscriptions
func (p *Player) UnsubscribeAll() {
	for name, unsub := range p.subs {
		unsub()
		delete(p.subs, name)
	}
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
	defer p.UnsubscribeAll()

	err := p.prompt()
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case msg := <-p.msgs:
			// Display NATS message to the player
			err = p.writeLine("\r\n" + string(msg))
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
			err = p.cmdHandler.Exec(ctx, &p.EntityState, cmdName, args...)
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
			if p.Quit {
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
	_, err := p.conn.Write([]byte(msg + "\r\n"))
	return err
}
