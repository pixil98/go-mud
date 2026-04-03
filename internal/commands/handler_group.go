package commands

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/display"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/shared"
)

// groupLeader returns the root group leader for actor, or nil if not in a group.
// Walks up the follow tree through grouped links to find the top-most leader.
func groupLeader(actor shared.Actor) game.FollowTarget {
	parent := actor.Following()
	if parent == nil || !parent.IsFollowerGrouped(actor.Id()) {
		if len(actor.GroupedFollowers()) > 0 {
			return actor
		}
		return nil
	}
	for i := 0; i < 100; i++ {
		next := parent.Following()
		if next == nil || !next.IsFollowerGrouped(parent.Id()) {
			return parent
		}
		parent = next
	}
	return parent
}

// disbandGroup removes all of the leader's direct grouped followers.
// Sub-groups remain intact — a sub-leader keeps their own grouped followers.
func disbandGroup(leader game.FollowTarget) {
	for _, ft := range leader.GroupedFollowers() {
		leader.SetFollowerGrouped(ft.Id(), false)
		ft.SetFollowing(nil)
		ft.Notify("The group has been disbanded.")
	}
	leader.Notify("You have disbanded the group.")
}

// ---------------------------------------------------------------------------
// GroupHandlerFactory
// ---------------------------------------------------------------------------

// GroupHandlerFactory creates handlers for the group command.
// With no target: displays the current group members.
// With a target: toggles membership — adds a following player who is not yet
// in the group, or removes one who already is.
type GroupHandlerFactory struct {
	pub game.Publisher
}

// NewGroupHandlerFactory creates a handler factory for group management commands.
func NewGroupHandlerFactory(pub game.Publisher) *GroupHandlerFactory {
	return &GroupHandlerFactory{pub: pub}
}

// Spec returns the handler's target and config requirements.
func (f *GroupHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypePlayer | targetTypeMobile, Required: false},
		},
	}
}

// ValidateConfig performs custom validation on the command config.
func (f *GroupHandlerFactory) ValidateConfig(config map[string]string) error { return nil }

// Create returns a compiled CommandFunc for this handler.
func (f *GroupHandlerFactory) Create() (CommandFunc, error) {
	return f.handle, nil
}

func (f *GroupHandlerFactory) handle(ctx context.Context, in *CommandInput) error {
	target := in.Targets["target"]
	if target == nil {
		return f.showGroup(in.Actor)
	}
	return f.toggleMember(in.Actor, target)
}

func (f *GroupHandlerFactory) showGroup(char shared.Actor) error {
	leader := groupLeader(char)
	if leader == nil {
		return NewUserError("You are not in a group.")
	}

	type memberLine struct {
		name   string
		line   string
		leader bool
	}

	formatMember := func(ft game.FollowTarget, isLeader bool) memberLine {
		label := "[Member]"
		if isLeader {
			label = "[Leader]"
		}
		currentHP, maxHP := ft.Resource(assets.ResourceHp)
		hp := fmt.Sprintf("%d/%d HP", currentHP, maxHP)
		return memberLine{
			name:   ft.Name(),
			line:   fmt.Sprintf("%-8s %-20s %s", label, ft.Name(), hp),
			leader: isLeader,
		}
	}

	var members []memberLine
	members = append(members, formatMember(leader, true))
	for _, ft := range leader.GroupedFollowers() {
		members = append(members, formatMember(ft, false))
	}

	sort.Slice(members, func(i, j int) bool {
		if members[i].leader != members[j].leader {
			return members[i].leader
		}
		return members[i].name < members[j].name
	})

	lines := make([]string, 0, len(members)+1)
	lines = append(lines, display.Colorize(display.Color.Cyan, "[ Group Members ]"))
	for _, m := range members {
		lines = append(lines, m.line)
	}
	char.Notify(strings.Join(lines, "\n"))
	return nil
}

func (f *GroupHandlerFactory) toggleMember(char shared.Actor, target *TargetRef) error {
	actorId := char.Id()
	targetId := target.Actor.CharId
	targetActor := target.Actor.Actor()

	// Toggle out: target is already grouped by actor.
	if char.IsFollowerGrouped(targetId) {
		char.SetFollowerGrouped(targetId, false)
		targetActor.SetFollowing(nil)

		targetActor.Notify(fmt.Sprintf("You have been removed from the group by %s.", char.Name()))
		if err := f.pub.Publish(game.GroupPublishTarget(char), nil,
			[]byte(fmt.Sprintf("%s has been removed from the group.", target.Actor.Name))); err != nil {
			slog.Warn("failed to notify group of removal", "error", err)
		}

		if len(char.GroupedFollowers()) == 0 {
			char.Notify("The group has been disbanded.")
		}
		return nil
	}

	// Toggle in: target must be following the actor and not already in a group.
	if targetId == actorId {
		return NewUserError("You are already in your own group.")
	}

	following := targetActor.Following()
	if following == nil || following.Id() != actorId {
		return NewUserError(fmt.Sprintf("%s is not following you.", target.Actor.Name))
	}

	if groupLeader(targetActor) != nil {
		return NewUserError(fmt.Sprintf("%s is already in a group.", target.Actor.Name))
	}

	if leader := groupLeader(char); leader != nil && leader.Id() != actorId {
		return NewUserError("Only the group leader can add members.")
	}

	char.SetFollowerGrouped(targetId, true)

	joinMsg := fmt.Sprintf("%s has joined the group.", target.Actor.Name)
	if err := f.pub.Publish(game.GroupPublishTarget(char), []string{targetId}, []byte(joinMsg)); err != nil {
		slog.Warn("failed to notify group of new member", "error", err)
	}
	targetActor.Notify(fmt.Sprintf("You join %s's group.", char.Name()))

	return nil
}

// ---------------------------------------------------------------------------
// UngroupHandlerFactory
// ---------------------------------------------------------------------------

// UngroupHandlerFactory creates handlers for the ungroup command.
// With no target: the leader disbands the group; a member leaves it.
// With a target: the leader removes a specific member.
// Targeting yourself disbands the group if you are the leader.
type UngroupHandlerFactory struct {
	pub game.Publisher
}

// NewUngroupHandlerFactory creates a handler factory for ungroup commands.
func NewUngroupHandlerFactory(pub game.Publisher) *UngroupHandlerFactory {
	return &UngroupHandlerFactory{pub: pub}
}

// Spec returns the handler's target and config requirements.
func (f *UngroupHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypePlayer | targetTypeMobile, Required: false},
		},
	}
}

// ValidateConfig performs custom validation on the command config.
func (f *UngroupHandlerFactory) ValidateConfig(config map[string]string) error { return nil }

// Create returns a compiled CommandFunc for this handler.
func (f *UngroupHandlerFactory) Create() (CommandFunc, error) {
	return f.handle, nil
}

func (f *UngroupHandlerFactory) handle(ctx context.Context, in *CommandInput) error {
	target := in.Targets["target"]
	if target == nil {
		return f.disbandOrLeave(in.Actor)
	}
	return f.removeTarget(in.Actor, target)
}

func (f *UngroupHandlerFactory) disbandOrLeave(char shared.Actor) error {
	leader := groupLeader(char)
	if leader == nil {
		return NewUserError("You are not in a group.")
	}

	if leader.Id() == char.Id() {
		disbandGroup(leader)
		return nil
	}

	// Non-leader leaves the group.
	leader.SetFollowerGrouped(char.Id(), false)
	char.SetFollowing(nil)

	char.Notify("You leave the group.")
	if err := f.pub.Publish(game.GroupPublishTarget(leader), nil,
		[]byte(fmt.Sprintf("%s has left the group.", char.Name()))); err != nil {
		slog.Warn("failed to notify group of member leaving", "error", err)
	}

	if len(leader.GroupedFollowers()) == 0 {
		leader.Notify("The group has been disbanded.")
	}
	return nil
}

func (f *UngroupHandlerFactory) removeTarget(char shared.Actor, target *TargetRef) error {
	leader := groupLeader(char)
	if leader == nil {
		return NewUserError("You are not in a group.")
	}
	if leader.Id() != char.Id() {
		return NewUserError("Only the group leader can remove members.")
	}

	targetId := target.Actor.CharId
	targetActor := target.Actor.Actor()

	// Targeting yourself disbands the whole group.
	if targetId == char.Id() {
		disbandGroup(leader)
		return nil
	}

	if !char.IsFollowerGrouped(targetId) {
		return NewUserError(fmt.Sprintf("%s is not in your group.", target.Actor.Name))
	}

	char.SetFollowerGrouped(targetId, false)
	targetActor.SetFollowing(nil)

	targetActor.Notify(fmt.Sprintf("You have been removed from the group by %s.", char.Name()))
	if err := f.pub.Publish(game.GroupPublishTarget(char), nil,
		[]byte(fmt.Sprintf("%s has been removed from the group.", target.Actor.Name))); err != nil {
		slog.Warn("failed to notify group of removal", "error", err)
	}

	if len(char.GroupedFollowers()) == 0 {
		char.Notify("The group has been disbanded.")
	}
	return nil
}
