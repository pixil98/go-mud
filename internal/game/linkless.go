package game

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/pixil98/go-mud/internal/storage"
)

const (
	DefaultLinklessTimeout = 5 * time.Minute
	DefaultIdleTimeout     = 15 * time.Minute
)

// SessionPublisher can send messages to rooms and individual players.
type SessionPublisher interface {
	PublishToRoom(zoneId, roomId storage.Identifier, data []byte) error
	PublishToPlayer(charId storage.Identifier, data []byte) error
}

// SessionTicker implements Ticker and handles idle kicks and linkless timeout removal.
type SessionTicker struct {
	world           *WorldState
	chars           storage.Storer[*Character]
	publisher       SessionPublisher
	linklessTimeout time.Duration
	idleTimeout     time.Duration
}

type SessionTickerOpt func(*SessionTicker)

func NewSessionTicker(world *WorldState, chars storage.Storer[*Character], pub SessionPublisher, opts ...SessionTickerOpt) *SessionTicker {
	st := &SessionTicker{
		world:           world,
		chars:           chars,
		publisher:       pub,
		linklessTimeout: DefaultLinklessTimeout,
		idleTimeout:     DefaultIdleTimeout,
	}
	for _, opt := range opts {
		opt(st)
	}
	return st
}

func WithLinklessTimeout(d time.Duration) SessionTickerOpt {
	return func(st *SessionTicker) {
		st.linklessTimeout = d
	}
}

func WithIdleTimeout(d time.Duration) SessionTickerOpt {
	return func(st *SessionTicker) {
		st.idleTimeout = d
	}
}

func (st *SessionTicker) Tick(ctx context.Context) error {
	now := time.Now()
	linklessCutoff := now.Add(-st.linklessTimeout)
	idleCutoff := now.Add(-st.idleTimeout)

	// Collect players needing action (ForEachPlayer holds a read lock, so act after).
	type action struct {
		charId   storage.Identifier
		linkless bool // true = linkless timeout, false = idle kick
	}
	var actions []action

	st.world.ForEachPlayer(func(id storage.Identifier, ps PlayerState) {
		if ps.Linkless && ps.LinklessAt.Before(linklessCutoff) {
			actions = append(actions, action{charId: id, linkless: true})
		} else if !ps.Linkless && ps.LastActivity.Before(idleCutoff) {
			actions = append(actions, action{charId: id, linkless: false})
		}
	})

	for _, a := range actions {
		ps := st.world.GetPlayer(a.charId)
		if ps == nil {
			continue
		}

		if a.linkless {
			// Linkless timeout: save, announce, and remove
			zoneId, roomId := ps.Location()
			ps.Character.LastZone = zoneId
			ps.Character.LastRoom = roomId
			if err := st.chars.Save(string(a.charId), ps.Character); err != nil {
				slog.ErrorContext(ctx, "saving linkless player", "charId", a.charId, "error", err)
			}

			if st.publisher != nil {
				msg := fmt.Sprintf("%s has lost %s link and fades from the world.",
					ps.Character.Name, ps.Character.Pronoun.Get().Possessive.Adjective)
				_ = st.publisher.PublishToRoom(zoneId, roomId, []byte(msg))
			}

			if err := st.world.RemovePlayer(a.charId); err != nil {
				slog.ErrorContext(ctx, "removing linkless player", "charId", a.charId, "error", err)
			}

			slog.InfoContext(ctx, "linkless player timed out", "charId", a.charId)
		} else {
			// Idle kick: notify and close connection — handleSessionEnd will mark linkless
			if st.publisher != nil {
				_ = st.publisher.PublishToPlayer(a.charId, []byte("You have been idle too long."))
			}
			ps.Kick()
			slog.InfoContext(ctx, "idle player kicked", "charId", a.charId)
		}
	}

	return nil
}
