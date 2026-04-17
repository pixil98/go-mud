package player

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/commands"
	"github.com/pixil98/go-mud/internal/game"
)

// Player represents an active player session, bridging the network connection to the game world.
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
	return p.charId
}

// Play runs the player's input/output loop until the connection closes, the player quits, or ctx is canceled.
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
	err := p.cmdHandler.Exec(ctx, p.world.GetPlayer(p.charId), "look")
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

	// Main play loop
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-p.done:
			// Drain any messages that arrived before or alongside the
			// done signal (e.g. the killing blow + death message).
			p.drainMessages()
			ps := p.world.GetPlayer(p.charId)
			if ps != nil && ps.IsQuit() {
				// Death or voluntary quit — return nil so handleSessionEnd saves and removes the player.
				return nil
			}
			var msg string
			if ps != nil && ps.IsLinkless() {
				msg = "Disconnected for inactivity."
			} else {
				msg = "Another connection has taken over your session."
			}
			if err := p.writeLine(msg); err != nil {
				slog.Warn("failed to write disconnect message to player", "charId", p.charId, "error", err)
			}
			return game.ErrPlayerReconnected

		case msg := <-p.msgs:
			// Write the message that woke us; drainMessages below
			// picks up any remaining.
			_ = p.writeLine(string(msg))

		case line, ok := <-inputChan:
			if !ok {
				return <-inputErrChan
			}

			p.world.MarkPlayerActive(p.charId)

			line = strings.TrimSpace(line)
			if line != "" {
				parts := strings.Fields(line)
				ps := p.world.GetPlayer(p.charId)
				if ps == nil {
					return fmt.Errorf("player state not found for %s", p.charId)
				}

				err = p.cmdHandler.Exec(ctx, ps, parts[0], parts[1:]...)
				if err != nil {
					var userErr *commands.UserError
					if errors.As(err, &userErr) {
						if err = p.writeLine(userErr.Message); err != nil {
							return err
						}
					} else {
						return fmt.Errorf("command execution failed: %w", err)
					}
				}

				if ps.IsQuit() {
					_ = p.writeLine("Goodbye!\n")
					return nil
				}
			}
		}

		// After every non-returning iteration, flush pending messages
		// and show the prompt.
		p.drainMessages()
		if err := p.prompt(); err != nil {
			return err
		}
	}
}

func (p *Player) prompt() error {
	prompt := "> "
	if ps := p.world.GetPlayer(p.charId); ps != nil {
		// Single pass: collect HP plus all other resources with their values
		// from ForEachResource directly, so we don't re-query each resource.
		type res struct {
			name     string
			cur, max int
		}
		var hp *res
		var others []res
		ps.ForEachResource(func(name string, cur, mx int) {
			if name == assets.ResourceHp {
				hp = &res{name, cur, mx}
				return
			}
			others = append(others, res{name, cur, mx})
		})
		sort.Slice(others, func(i, j int) bool { return others[i].name < others[j].name })

		var parts []string
		if hp != nil && hp.max > 0 {
			parts = append(parts, game.ResourceLine(hp.name, hp.cur, hp.max))
		}
		for _, r := range others {
			parts = append(parts, game.ResourceLine(r.name, r.cur, r.max))
		}
		if ap := ps.CurrentAP(); ap > 0 {
			parts = append(parts, strings.Repeat("*", ap))
		}
		if len(parts) > 0 {
			prompt = fmt.Sprintf("[%s] > ", strings.Join(parts, " | "))
		}
	}
	_, err := p.conn.Write([]byte("\n\n" + prompt))
	return err
}

func (p *Player) drainMessages() {
	for {
		select {
		case msg := <-p.msgs:
			_ = p.writeLine(string(msg))
		default:
			// Channel empty — wait briefly for in-flight messages
			// (e.g. NATS deliveries from the same tick) before returning.
			select {
			case msg := <-p.msgs:
				_ = p.writeLine(string(msg))
			case <-time.After(5 * time.Millisecond):
				return
			}
		}
	}
}

func (p *Player) writeLine(msg string) error {
	_, err := p.conn.Write([]byte("\n" + msg))
	return err
}
