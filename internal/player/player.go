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

	// Subscriber for creating new subscriptions
	subscriber Subscriber
	// Cleanup functions for active subscriptions (player, room, group, etc.)
	subs map[string]func()
	msgs chan []byte
}

// Id returns the player's unique identifier (lowercase character name)
func (p *Player) Id() string {
	return string(p.charId)
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

			// Capture location before command execution
			state := p.world.GetPlayer(p.charId)
			if state == nil {
				return fmt.Errorf("player state not found for %s", p.charId)
			}
			prevZone, prevRoom := state.Location()

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
			state = p.world.GetPlayer(p.charId)
			if state == nil {
				return fmt.Errorf("player state not found for %s", p.charId)
			}
			if state.IsQuitting() {
				p.writeLine("Goodbye!")
				return nil
			}

			// Update subscriptions if location changed
			// TODO: Consider moving subscription management to WorldState so that
			// features like "follow" can automatically update follower subscriptions
			// when a player moves.
			curZone, curRoom := state.Location()
			if curZone != prevZone {
				p.Unsubscribe("zone")
				zoneSubject := fmt.Sprintf("zone-%s", curZone)
				if err := p.Subscribe("zone", zoneSubject); err != nil {
					return fmt.Errorf("subscribing to zone channel: %w", err)
				}
			}
			if curZone != prevZone || curRoom != prevRoom {
				p.Unsubscribe("room")
				roomSubject := fmt.Sprintf("zone-%s-room-%s", curZone, curRoom)
				if err := p.Subscribe("room", roomSubject); err != nil {
					return fmt.Errorf("subscribing to room channel: %w", err)
				}
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
