package player

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"unicode"

	"github.com/pixil98/go-log/log"
)

type State struct {
	mu   sync.Mutex
	char *Character
}

type Flow interface {
	Name() string
	Run(string, *State, io.Writer) (bool, error)
}

type Player struct {
	conn      io.ReadWriter
	state     *State
	loggedIn  bool
	flow      Flow
	loginFlow Flow
	mainFlow  Flow
}

func (p *Player) Tick(ctx context.Context) {
	// do something like regen here
	//p.character.Regen()
}

func (p *Player) Play(ctx context.Context) error {
	r := bufio.NewReader(p.conn)

	var str string
	for {

		fmt.Println(p.flow)
		// Run the next step in the current flow
		done, err := p.flow.Run(str, p.state, p.conn)
		if err != nil {
			return fmt.Errorf("running flow %s: %w", p.flow.Name(), err)
		}
		fmt.Println("here 0")

		if done {
			fmt.Println("done")
			if !p.loggedIn {
				p.loggedIn = true
				p.flow = p.mainFlow
				continue
			} else {
				return nil
			}
		}
		fmt.Println("here 1")

		// Get the next input from the player
		line, _, err := r.ReadLine()
		if err != nil {
			if err.Error() == "EOF" {
				return nil
			} else {
				return fmt.Errorf("reading connection: %w", err)
			}
		}

		fmt.Println("here 2")

		// clean up the input
		str = strings.TrimSpace(string(line))
		// strip non printable characters
		str = strings.Map(func(r rune) rune {
			if unicode.IsGraphic(r) {
				return r
			}
			return -1
		}, str)

		log.GetLogger(ctx).Infof("received input: %s", str)
	}
}
