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
)

// disbandGroup removes all of the leader's direct grouped followers.
// Sub-groups remain intact — a sub-leader keeps their own grouped followers.
func disbandGroup(leader game.Actor) {
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
	targets := in.Targets["target"]
	if len(targets) == 0 {
		return f.showGroup(in.Actor)
	}
	for _, target := range targets {
		if err := f.toggleMember(in.Actor, target); err != nil {
			return err
		}
	}
	return nil
}

func (f *GroupHandlerFactory) showGroup(char game.Actor) error {
	leader := game.GroupLeader(char)
	if leader == nil {
		return NewUserError("You are not in a group.")
	}

	type entry struct {
		actor  game.Actor
		label  string // role suffix like "(Leader)"
		indent string // tree drawing prefix, e.g. "  │  └─ "
	}

	// Walk the tree depth-first, building display entries.
	var entries []entry
	var walk func(actor game.Actor, label, prefix, childPrefix string)
	walk = func(actor game.Actor, label, prefix, childPrefix string) {
		entries = append(entries, entry{actor: actor, label: label, indent: prefix})
		followers := actor.GroupedFollowers()
		for i, f := range followers {
			isLast := i == len(followers)-1
			var conn, nextChildPrefix string
			if isLast {
				conn = childPrefix + "└─ "
				nextChildPrefix = childPrefix + "   "
			} else {
				conn = childPrefix + "├─ "
				nextChildPrefix = childPrefix + "│  "
			}
			walk(f, "", conn, nextChildPrefix)
		}
	}

	entries = append(entries, entry{actor: leader, label: "(Leader)"})
	for _, ft := range leader.GroupedFollowers() {
		walk(ft, "", "", "")
	}

	// Compute max display name width for padding.
	maxNameWidth := 0
	for _, e := range entries {
		name := e.indent + e.actor.Name()
		if e.label != "" {
			name += " " + e.label
		}
		if len(name) > maxNameWidth {
			maxNameWidth = len(name)
		}
	}

	// Format each line.
	lines := []string{display.Colorize(display.Color.Cyan, "[ Group Members ]")}
	for _, e := range entries {
		name := e.indent + e.actor.Name()
		if e.label != "" {
			name += " " + e.label
		}

		type res struct {
			name    string
			current int
			max     int
		}
		var resources []res
		e.actor.ForEachResource(func(name string, current, maximum int) {
			resources = append(resources, res{name, current, maximum})
		})
		sort.Slice(resources, func(i, j int) bool {
			iHP := resources[i].name == assets.ResourceHp
			jHP := resources[j].name == assets.ResourceHp
			if iHP != jHP {
				return iHP
			}
			return resources[i].name < resources[j].name
		})
		var resParts []string
		for _, r := range resources {
			resParts = append(resParts, fmt.Sprintf("%s: %d/%d", r.name, r.current, r.max))
		}

		line := fmt.Sprintf("%-*s  %s", maxNameWidth, name, strings.Join(resParts, " | "))
		lines = append(lines, line)
	}
	char.Notify(strings.Join(lines, "\n"))
	return nil
}

func (f *GroupHandlerFactory) toggleMember(char game.Actor, target *TargetRef) error {
	actorId := char.Id()
	targetActor := target.Actor.Actor()
	targetId := targetActor.Id()

	if char.IsFollowerGrouped(targetId) {
		return NewUserError(fmt.Sprintf("%s is already in your group.", target.Actor.Name))
	}

	// Add to group: target must be following the actor and not already in a group.
	if targetId == actorId {
		return NewUserError("You are already in your own group.")
	}

	following := targetActor.Following()
	if following == nil || following.Id() != actorId {
		return NewUserError(fmt.Sprintf("%s is not following you.", target.Actor.Name))
	}

	if game.GroupLeader(targetActor) != nil {
		return NewUserError(fmt.Sprintf("%s is already in a group.", target.Actor.Name))
	}

	char.SetFollowerGrouped(targetId, true)

	// Announce to the wider group if the actor is part of one.
	leader := game.GroupLeader(char)
	if leader != nil {
		joinMsg := fmt.Sprintf("%s has joined the group.", target.Actor.Name)
		if err := f.pub.Publish(game.GroupPublishTarget(leader), []string{targetId}, []byte(joinMsg)); err != nil {
			slog.Warn("failed to notify group of new member", "error", err)
		}
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
	targets := in.Targets["target"]
	if len(targets) == 0 {
		return f.disbandOrLeave(in.Actor)
	}
	for _, target := range targets {
		if err := f.removeTarget(in.Actor, target); err != nil {
			return err
		}
	}
	return nil
}

func (f *UngroupHandlerFactory) disbandOrLeave(char game.Actor) error {
	leader := game.GroupLeader(char)
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

func (f *UngroupHandlerFactory) removeTarget(char game.Actor, target *TargetRef) error {
	leader := game.GroupLeader(char)
	if leader == nil {
		return NewUserError("You are not in a group.")
	}
	if leader.Id() != char.Id() {
		return NewUserError("Only the group leader can remove members.")
	}

	targetActor := target.Actor.Actor()
	targetId := targetActor.Id()

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
