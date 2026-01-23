package player

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/pixil98/go-mud/internal/commands"
	"github.com/pixil98/go-mud/internal/game"
)

type Player struct {
	mu         sync.Mutex
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

	p.prompt()

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			p.prompt()
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
		err := p.cmdHandler.Exec(ctx, &p.EntityState, cmdName, args...)
		if err != nil {
			p.writeLine(fmt.Sprintf("Error: %v", err))
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

		p.prompt()
	}

	return scanner.Err()
}

func (p *Player) prompt() {
	p.conn.Write([]byte("> "))
}

func (p *Player) writeLine(msg string) {
	p.conn.Write([]byte(msg + "\r\n"))
}
