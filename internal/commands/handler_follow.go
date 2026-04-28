package commands

import (
	"context"
	"fmt"

	"github.com/pixil98/go-mud/internal/game"
)

// FollowActor provides the state needed by the follow handler.
type FollowActor interface {
	Id() string
	Name() string
	Publish(data []byte, exclude []string)
	Following() game.Actor
	SetFollowing(game.Actor)
}

var _ FollowActor = (*game.CharacterInstance)(nil)

// FollowHandlerFactory creates handlers for the follow and unfollow commands.
// When a target is resolved (follow command), the player starts following that target.
// When no target is present (unfollow command), the player stops following.
type FollowHandlerFactory struct{}

// NewFollowHandlerFactory creates a handler factory for follow and unfollow commands.
func NewFollowHandlerFactory() *FollowHandlerFactory {
	return &FollowHandlerFactory{}
}

// Spec returns the handler's target and config requirements.
func (f *FollowHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypePlayer | targetTypeMobile, Required: false},
		},
	}
}

// ValidateConfig performs custom validation on the command config.
func (f *FollowHandlerFactory) ValidateConfig(config map[string]string) error {
	return nil
}

// Create returns a compiled CommandFunc for this handler.
func (f *FollowHandlerFactory) Create() (CommandFunc, error) {
	return Adapt[FollowActor](f.handle), nil
}

func (f *FollowHandlerFactory) handle(ctx context.Context, char FollowActor, in *CommandInput) error {
	target := in.FirstTarget("target")
	if target == nil {
		return f.unfollow(char)
	}
	return f.follow(char, target)
}

func (f *FollowHandlerFactory) follow(char FollowActor, target *TargetRef) error {
	leader := target.Actor.Actor()

	if leader.Id() == char.Id() {
		return NewUserError("You can't follow yourself.")
	}

	if cur := char.Following(); cur != nil && cur.Id() == leader.Id() {
		return NewUserError(fmt.Sprintf("You are already following %s.", target.Actor.Name))
	}

	if wouldCreateLoop(char.Id(), leader) {
		return NewUserError("Sorry, following in loops is not allowed.")
	}

	// Stop following old leader first.
	if old := char.Following(); old != nil {
		old.Publish([]byte(fmt.Sprintf("%s stops following you.", char.Name())), nil)
		char.Publish([]byte(fmt.Sprintf("You stop following %s.", old.Name())), nil)
	}

	char.SetFollowing(leader)

	char.Publish([]byte(fmt.Sprintf("You now follow %s.", target.Actor.Name)), nil)
	leader.Publish([]byte(fmt.Sprintf("%s now follows you.", char.Name())), nil)

	return nil
}

func (f *FollowHandlerFactory) unfollow(char FollowActor) error {
	old := char.Following()
	if old == nil {
		return NewUserError("You aren't following anyone.")
	}

	char.Publish([]byte(fmt.Sprintf("You stop following %s.", old.Name())), nil)
	old.Publish([]byte(fmt.Sprintf("%s stops following you.", char.Name())), nil)
	char.SetFollowing(nil)
	return nil
}

// wouldCreateLoop returns true if making followerId follow leader would
// create a circular follow chain.
func wouldCreateLoop(followerId string, leader game.Actor) bool {
	current := leader
	for i := 0; i < 100; i++ {
		next := current.Following()
		if next == nil {
			return false
		}
		if next.Id() == followerId {
			return true
		}
		current = next
	}
	return true
}
