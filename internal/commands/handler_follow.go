package commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pixil98/go-mud/internal/game"
)

// FollowActor provides the character state needed by the follow handler.
type FollowActor interface {
	CommandActor
	GetFollowingId() string
	SetFollowingId(string)
}

var _ FollowActor = (*game.CharacterInstance)(nil)

// FollowedPlayer provides the state the follow handler reads from a looked-up player.
type FollowedPlayer interface {
	Name() string
	GetFollowingId() string
}

var _ FollowedPlayer = (*game.CharacterInstance)(nil)

// FollowPlayerLookup finds players for the follow handler.
type FollowPlayerLookup interface {
	GetPlayer(charId string) FollowedPlayer
}

// followPlayerAdapter wraps a PlayerLookup to satisfy FollowPlayerLookup.
type followPlayerAdapter struct {
	inner PlayerLookup
}

func (a *followPlayerAdapter) GetPlayer(charId string) FollowedPlayer {
	p := a.inner.GetPlayer(charId)
	if p == nil {
		return nil
	}
	return p
}

// FollowHandlerFactory creates handlers for the follow and unfollow commands.
// When a target is resolved (follow command), the player starts following that target.
// When no target is present (unfollow command), the player stops following.
type FollowHandlerFactory struct {
	players FollowPlayerLookup
	pub     game.Publisher
}

func NewFollowHandlerFactory(players PlayerLookup, pub game.Publisher) *FollowHandlerFactory {
	return &FollowHandlerFactory{players: &followPlayerAdapter{inner: players}, pub: pub}
}

func (f *FollowHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypePlayer, Required: false},
		},
	}
}

func (f *FollowHandlerFactory) ValidateConfig(config map[string]string) error {
	return nil
}

func (f *FollowHandlerFactory) Create() (CommandFunc, error) {
	return Adapt[FollowActor](f.handle), nil
}

func (f *FollowHandlerFactory) handle(ctx context.Context, char FollowActor, in *CommandInput) error {
	target := in.Targets["target"]
	if target == nil {
		return f.unfollow(char)
	}
	return f.follow(char, target)
}

func (f *FollowHandlerFactory) follow(char FollowActor, target *TargetRef) error {
	actorId := char.Id()
	leaderId := target.Player.CharId

	// Can't follow yourself.
	if leaderId == actorId {
		return NewUserError("You can't follow yourself.")
	}

	// Already following this person.
	if char.GetFollowingId() == leaderId {
		return NewUserError(fmt.Sprintf("You are already following %s.", target.Player.Name))
	}

	// Prevent circular follows.
	if wouldCreateLoop(f.players, actorId, leaderId) {
		return NewUserError("Sorry, following in loops is not allowed.")
	}

	// Stop following old leader first.
	if char.GetFollowingId() != "" {
		f.notifyStopFollowing(char)
	}

	char.SetFollowingId(leaderId)

	// Notify both parties.
	if err := f.pub.Publish(game.SinglePlayer(actorId), nil,
		[]byte(fmt.Sprintf("You now follow %s.", target.Player.Name))); err != nil {
		slog.Warn("failed to notify follower", "error", err)
	}
	if err := f.pub.Publish(game.SinglePlayer(leaderId), nil,
		[]byte(fmt.Sprintf("%s now follows you.", char.Name()))); err != nil {
		slog.Warn("failed to notify leader", "error", err)
	}

	return nil
}

func (f *FollowHandlerFactory) unfollow(char FollowActor) error {
	if char.GetFollowingId() == "" {
		return NewUserError("You aren't following anyone.")
	}

	f.notifyStopFollowing(char)
	char.SetFollowingId("")
	return nil
}

// notifyStopFollowing sends stop-following messages to both parties and does NOT
// clear FollowingId — the caller is responsible for that.
func (f *FollowHandlerFactory) notifyStopFollowing(char FollowActor) {
	actorId := char.Id()
	leaderId := char.GetFollowingId()

	leaderPs := f.players.GetPlayer(leaderId)
	if leaderPs != nil {
		leaderName := leaderPs.Name()
		if err := f.pub.Publish(game.SinglePlayer(actorId), nil,
			[]byte(fmt.Sprintf("You stop following %s.", leaderName))); err != nil {
			slog.Warn("failed to notify follower", "error", err)
		}
		if err := f.pub.Publish(game.SinglePlayer(leaderId), nil,
			[]byte(fmt.Sprintf("%s stops following you.", char.Name()))); err != nil {
			slog.Warn("failed to notify leader", "error", err)
		}
	}
}

// wouldCreateLoop returns true if making followerId follow leaderId would
// create a circular follow chain.
func wouldCreateLoop(players FollowPlayerLookup, followerId, leaderId string) bool {
	current := leaderId
	for i := 0; i < 100; i++ {
		ps := players.GetPlayer(current)
		if ps == nil || ps.GetFollowingId() == "" {
			return false
		}
		if ps.GetFollowingId() == followerId {
			return true
		}
		current = ps.GetFollowingId()
	}
	// Safety: if we walked 100 links, treat as a loop.
	return true
}
