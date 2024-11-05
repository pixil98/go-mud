package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/storage"
)

type handlerFunc func(context.Context, ...string) error

type Handler struct {
	store    storage.Storer[*Command]
	handlers map[string]handlerFunc
}

func NewHandler(c storage.Storer[*Command]) *Handler {
	h := &Handler{
		store: c,
	}
	h.handlers = map[string]handlerFunc{
		"message": h.do_message,
	}
	return h
}

func (h *Handler) Exec(ctx context.Context, c string, args ...string) error {
	cmd := h.store.Get(c)
	if cmd == nil {
		return nil
	}

	if cmd.Handler == "" {
		return fmt.Errorf("command handler not set")
	}

	fn, ok := h.handlers[cmd.Handler]
	if !ok {
		return fmt.Errorf("handler not found: %s", cmd.Handler)
	}

	// Build up the args
	var args []string
	for _, a := range cmd.Args {

		args = append(args, a)
	}

	err := fn(ctx, args...)
	if err != nil {
		return fmt.Errorf("executing handler: %w", err)
	}

	return nil
}

func (h *Handler) parseArgs(args ...string) ([]string, error) {
	for _, a := range args {
		
		
}

func (h *Handler) do_message(ctx context.Context, args ...string) error {
	fmt.Println("message:")
	for _, a := range args {
		fmt.Printf("    %s", a)
	}
	return nil
}
