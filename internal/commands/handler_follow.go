package commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pixil98/go-mud/internal/game"
)

// FollowHandlerFactory creates handlers for the follow and unfollow commands.
// When a target is resolved (follow command), the player starts following that target.
// When no target is present (unfollow command), the player stops following.
type FollowHandlerFactory struct {
	players PlayerLookup
	pub     game.Publisher
}

func NewFollowHandlerFactory(players PlayerLookup, pub game.Publisher) *FollowHandlerFactory {
	return &FollowHandlerFactory{players: players, pub: pub}
}

func (f *FollowHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypePlayer, Required: false},
		},
	}
}

func (f *FollowHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *FollowHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, in *CommandInput) error {
		target := in.Targets["target"]
		if target == nil {
			return f.unfollow(in)
		}
		return f.follow(in, target)
	}, nil
}

func (f *FollowHandlerFactory) follow(in *CommandInput, target *TargetRef) error {
	actorId := in.Char.Character.Id()
	leaderId := target.Player.CharId

	// Can't follow yourself.
	if leaderId == actorId {
		return NewUserError("You can't follow yourself.")
	}

	// Already following this person.
	if in.Char.FollowingId == leaderId {
		return NewUserError(fmt.Sprintf("You are already following %s.", target.Player.Name))
	}

	// Prevent circular follows.
	if wouldCreateLoop(f.players, actorId, leaderId) {
		return NewUserError("Sorry, following in loops is not allowed.")
	}

	// Stop following old leader first.
	if in.Char.FollowingId != "" {
		f.notifyStopFollowing(in)
	}

	in.Char.FollowingId = leaderId

	// Notify both parties.
	if err := f.pub.Publish(game.SinglePlayer(actorId), nil,
		[]byte(fmt.Sprintf("You now follow %s.", target.Player.Name))); err != nil {
		slog.Warn("failed to notify follower", "error", err)
	}
	if err := f.pub.Publish(game.SinglePlayer(leaderId), nil,
		[]byte(fmt.Sprintf("%s now follows you.", in.Char.Character.Get().Name))); err != nil {
		slog.Warn("failed to notify leader", "error", err)
	}

	return nil
}

func (f *FollowHandlerFactory) unfollow(in *CommandInput) error {
	if in.Char.FollowingId == "" {
		return NewUserError("You aren't following anyone.")
	}

	f.notifyStopFollowing(in)
	in.Char.FollowingId = ""
	return nil
}

// notifyStopFollowing sends stop-following messages to both parties and does NOT
// clear FollowingId — the caller is responsible for that.
func (f *FollowHandlerFactory) notifyStopFollowing(in *CommandInput) {
	actorId := in.Char.Character.Id()
	leaderId := in.Char.FollowingId

	leaderPs := f.players.GetPlayer(leaderId)
	if leaderPs != nil {
		leaderName := leaderPs.Character.Get().Name
		if err := f.pub.Publish(game.SinglePlayer(actorId), nil,
			[]byte(fmt.Sprintf("You stop following %s.", leaderName))); err != nil {
			slog.Warn("failed to notify follower", "error", err)
		}
		if err := f.pub.Publish(game.SinglePlayer(leaderId), nil,
			[]byte(fmt.Sprintf("%s stops following you.", in.Char.Character.Get().Name))); err != nil {
			slog.Warn("failed to notify leader", "error", err)
		}
	}
}

// wouldCreateLoop returns true if making followerId follow leaderId would
// create a circular follow chain.
func wouldCreateLoop(players PlayerLookup, followerId, leaderId string) bool {
	current := leaderId
	for i := 0; i < 100; i++ {
		ps := players.GetPlayer(current)
		if ps == nil || ps.FollowingId == "" {
			return false
		}
		if ps.FollowingId == followerId {
			return true
		}
		current = ps.FollowingId
	}
	// Safety: if we walked 100 links, treat as a loop.
	return true
}
