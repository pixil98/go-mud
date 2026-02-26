package commands

import (
	"context"
	"fmt"

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
			{Name: "target", Type: TargetTypePlayer, Required: false},
		},
	}
}

func (f *FollowHandlerFactory) ValidateConfig(config map[string]any) error {
	return nil
}

func (f *FollowHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		target := cmdCtx.Targets["target"]
		if target == nil {
			return f.unfollow(cmdCtx)
		}
		return f.follow(cmdCtx, target)
	}, nil
}

func (f *FollowHandlerFactory) follow(cmdCtx *CommandContext, target *TargetRef) error {
	actorId := cmdCtx.Session.Character.Id()
	leaderId := target.Player.CharId

	// Can't follow yourself.
	if leaderId == actorId {
		return NewUserError("You can't follow yourself.")
	}

	// Already following this person.
	if cmdCtx.Session.FollowingId == leaderId {
		return NewUserError(fmt.Sprintf("You are already following %s.", target.Player.Name))
	}

	// Prevent circular follows.
	if wouldCreateLoop(f.players, actorId, leaderId) {
		return NewUserError("Sorry, following in loops is not allowed.")
	}

	// Stop following old leader first.
	if cmdCtx.Session.FollowingId != "" {
		f.notifyStopFollowing(cmdCtx)
	}

	cmdCtx.Session.FollowingId = leaderId

	// Notify both parties.
	_ = f.pub.Publish(game.SinglePlayer(actorId), nil,
		[]byte(fmt.Sprintf("You now follow %s.", target.Player.Name)))
	_ = f.pub.Publish(game.SinglePlayer(leaderId), nil,
		[]byte(fmt.Sprintf("%s now follows you.", cmdCtx.Actor.Name)))

	return nil
}

func (f *FollowHandlerFactory) unfollow(cmdCtx *CommandContext) error {
	if cmdCtx.Session.FollowingId == "" {
		return NewUserError("You aren't following anyone.")
	}

	f.notifyStopFollowing(cmdCtx)
	cmdCtx.Session.FollowingId = ""
	return nil
}

// notifyStopFollowing sends stop-following messages to both parties and does NOT
// clear FollowingId — the caller is responsible for that.
func (f *FollowHandlerFactory) notifyStopFollowing(cmdCtx *CommandContext) {
	actorId := cmdCtx.Session.Character.Id()
	leaderId := cmdCtx.Session.FollowingId

	leaderPs := f.players.GetPlayer(leaderId)
	if leaderPs != nil {
		leaderName := leaderPs.Character.Get().Name
		_ = f.pub.Publish(game.SinglePlayer(actorId), nil,
			[]byte(fmt.Sprintf("You stop following %s.", leaderName)))
		_ = f.pub.Publish(game.SinglePlayer(leaderId), nil,
			[]byte(fmt.Sprintf("%s stops following you.", cmdCtx.Actor.Name)))
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
