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
}

func (p *Player) Tick(ctx context.Context) {
	// do something like regen here
	//p.character.Regen()
}

func (p *Player) Play(ctx context.Context) error {
	scanner := bufio.NewScanner(p.conn)

	err := p.prompt()
	if err != nil {
		return err
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
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

		// Check if context was canceled
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err = p.prompt()
		if err != nil {
			return err
		}
	}

	return scanner.Err()
}

func (p *Player) prompt() error {
	_, err := p.conn.Write([]byte("> "))
	return err
}

func (p *Player) writeLine(msg string) error {
	_, err := p.conn.Write([]byte(msg + "\r\n"))
	return err
}
