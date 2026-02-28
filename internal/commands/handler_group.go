package commands

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/pixil98/go-mud/internal/display"
	"github.com/pixil98/go-mud/internal/game"
)

// groupCore holds the shared dependencies and operations used by both the
// group and ungroup command handlers.
type groupCore struct {
	players PlayerLookup
	pub     game.Publisher
}

// clearFollow clears ps.Group and, if they were following leaderId, clears
// FollowingId. Returns true if a follow was broken. Safe to call inside
// ForEachPlayer because it only mutates PlayerState fields, not the Group.
func clearFollow(leaderId string, ps *game.PlayerState) bool {
	if ps == nil {
		return false
	}
	ps.Group = nil
	if ps.FollowingId == leaderId {
		ps.FollowingId = ""
		return true
	}
	return false
}

// removeMember removes targetId from grp, clears their state via clearFollow,
// stops their follow if applicable, and notifies all parties.
// It does NOT auto-disband — callers decide whether to do that afterward.
func (c *groupCore) removeMember(leaderName, leaderId, targetId, targetName string, targetPs *game.PlayerState, grp *game.Group) {
	grp.RemoveMember(targetId)
	if clearFollow(leaderId, targetPs) {
		if err := c.pub.Publish(game.SinglePlayer(targetId), nil,
			[]byte(fmt.Sprintf("You stop following %s.", leaderName))); err != nil {
			slog.Warn("failed to notify follower", "error", err)
		}
		if err := c.pub.Publish(game.SinglePlayer(leaderId), nil,
			[]byte(fmt.Sprintf("%s stops following you.", targetName))); err != nil {
			slog.Warn("failed to notify leader", "error", err)
		}
	}
	if err := c.pub.Publish(game.SinglePlayer(targetId), nil,
		[]byte(fmt.Sprintf("You have been removed from the group by %s.", leaderName))); err != nil {
		slog.Warn("failed to notify removed member", "error", err)
	}
	if err := c.pub.Publish(grp, nil,
		[]byte(fmt.Sprintf("%s has been removed from the group.", targetName))); err != nil {
		slog.Warn("failed to notify group of removal", "error", err)
	}
}

// disband dissolves the group, clearing every member's state and notifying them.
func (c *groupCore) disband(grp *game.Group) {
	leaderId := grp.LeaderId
	grp.ForEachPlayer(func(charId string, ps *game.PlayerState) {
		clearFollow(leaderId, ps)
		msg := "The group has been disbanded."
		if charId == leaderId {
			msg = "You have disbanded the group."
		}
		if err := c.pub.Publish(game.SinglePlayer(charId), nil, []byte(msg)); err != nil {
			slog.Warn("failed to notify member of disband", "error", err)
		}
	})
}

// soloLeader reports whether the group now contains only the leader.
func soloLeader(grp *game.Group) bool {
	count := 0
	grp.ForEachPlayer(func(_ string, _ *game.PlayerState) { count++ })
	return count == 1
}

// ---------------------------------------------------------------------------
// GroupHandlerFactory
// ---------------------------------------------------------------------------

// GroupHandlerFactory creates handlers for the group command.
// With no target: displays the current group members.
// With a target: toggles membership — adds a following player who is not yet
// in the group, or removes one who already is.
type GroupHandlerFactory struct {
	groupCore
}

func NewGroupHandlerFactory(players PlayerLookup, pub game.Publisher) *GroupHandlerFactory {
	return &GroupHandlerFactory{groupCore{players: players, pub: pub}}
}

func (f *GroupHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: TargetTypePlayer, Required: false},
		},
	}
}

func (f *GroupHandlerFactory) ValidateConfig(config map[string]any) error { return nil }

func (f *GroupHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		target := cmdCtx.Targets["target"]
		if target == nil {
			return f.showGroup(cmdCtx)
		}
		return f.toggleMember(cmdCtx, target)
	}, nil
}

func (f *GroupHandlerFactory) showGroup(cmdCtx *CommandContext) error {
	actorId := cmdCtx.Session.Character.Id()
	grp := cmdCtx.Session.Group
	if grp == nil {
		return NewUserError("You are not in a group.")
	}

	type memberLine struct {
		name   string
		line   string
		leader bool
	}
	var members []memberLine

	grp.ForEachPlayer(func(_ string, ps *game.PlayerState) {
		if ps == nil {
			return
		}
		char := ps.Character.Get()
		isLeader := ps.Character.Id() == grp.LeaderId
		label := "[Member]"
		if isLeader {
			label = "[Leader]"
		}
		hp := fmt.Sprintf("%d/%d HP", char.CurrentHP, char.MaxHP)
		members = append(members, memberLine{
			name:   char.Name,
			line:   fmt.Sprintf("%-8s %-20s %s", label, char.Name, hp),
			leader: isLeader,
		})
	})

	// Leader first, then alphabetical.
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
	return f.pub.Publish(game.SinglePlayer(actorId), nil, []byte(strings.Join(lines, "\n")))
}

func (f *GroupHandlerFactory) toggleMember(cmdCtx *CommandContext, target *TargetRef) error {
	actorId := cmdCtx.Session.Character.Id()
	targetId := target.Player.CharId

	targetPs := f.players.GetPlayer(targetId)
	if targetPs == nil {
		return NewUserError("They are not available.")
	}

	grp := cmdCtx.Session.Group

	// Toggle out: target is already in this group — remove them.
	if grp != nil && grp.HasMember(targetId) {
		if grp.LeaderId != actorId {
			return NewUserError("Only the group leader can remove members.")
		}
		f.removeMember(cmdCtx.Actor.Name, actorId, targetId, target.Player.Name, targetPs, grp)
		if soloLeader(grp) {
			f.disband(grp)
		}
		return nil
	}

	// Toggle in: target must be following the actor and not in another group.
	if targetId == actorId {
		return NewUserError("You are already in your own group.")
	}
	if targetPs.FollowingId != actorId {
		return NewUserError(fmt.Sprintf("%s is not following you.", target.Player.Name))
	}
	if targetPs.Group != nil {
		return NewUserError(fmt.Sprintf("%s is already in a group.", target.Player.Name))
	}
	if grp != nil && grp.LeaderId != actorId {
		return NewUserError("Only the group leader can add members.")
	}

	if grp == nil {
		grp = game.NewGroup(actorId, cmdCtx.Session)
		cmdCtx.Session.Group = grp
	}

	grp.AddMember(targetId, targetPs)
	targetPs.Group = grp

	joinMsg := fmt.Sprintf("%s has joined the group.", target.Player.Name)
	if err := f.pub.Publish(grp, []string{targetId}, []byte(joinMsg)); err != nil {
		slog.Warn("failed to notify group of new member", "error", err)
	}
	if err := f.pub.Publish(game.SinglePlayer(targetId), nil,
		[]byte(fmt.Sprintf("You join %s's group.", cmdCtx.Actor.Name))); err != nil {
		slog.Warn("failed to notify new member", "error", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// UngroupHandlerFactory
// ---------------------------------------------------------------------------

// UngroupHandlerFactory creates handlers for the ungroup command.
// With no target: the leader disbands the group; a member leaves it.
// With a target: the leader removes a specific member (also stops their follow).
//   Targeting yourself disbands the group if you are the leader.
type UngroupHandlerFactory struct {
	groupCore
}

func NewUngroupHandlerFactory(players PlayerLookup, pub game.Publisher) *UngroupHandlerFactory {
	return &UngroupHandlerFactory{groupCore{players: players, pub: pub}}
}

func (f *UngroupHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: TargetTypePlayer, Required: false},
		},
	}
}

func (f *UngroupHandlerFactory) ValidateConfig(config map[string]any) error { return nil }

func (f *UngroupHandlerFactory) Create() (CommandFunc, error) {
	return func(ctx context.Context, cmdCtx *CommandContext) error {
		target := cmdCtx.Targets["target"]
		if target == nil {
			return f.disbandOrLeave(cmdCtx)
		}
		return f.removeTarget(cmdCtx, target)
	}, nil
}

func (f *UngroupHandlerFactory) disbandOrLeave(cmdCtx *CommandContext) error {
	actorId := cmdCtx.Session.Character.Id()
	grp := cmdCtx.Session.Group
	if grp == nil {
		return NewUserError("You are not in a group.")
	}

	if grp.LeaderId == actorId {
		f.disband(grp)
		return nil
	}

	// Non-leader leaves the group.
	grp.RemoveMember(actorId)
	_ = clearFollow(grp.LeaderId, f.players.GetPlayer(actorId))

	if err := f.pub.Publish(game.SinglePlayer(actorId), nil, []byte("You leave the group.")); err != nil {
		slog.Warn("failed to notify leaving member", "error", err)
	}
	if err := f.pub.Publish(grp, nil,
		[]byte(fmt.Sprintf("%s has left the group.", cmdCtx.Actor.Name))); err != nil {
		slog.Warn("failed to notify group of member leaving", "error", err)
	}

	if soloLeader(grp) {
		f.disband(grp)
	}
	return nil
}

func (f *UngroupHandlerFactory) removeTarget(cmdCtx *CommandContext, target *TargetRef) error {
	actorId := cmdCtx.Session.Character.Id()
	targetId := target.Player.CharId
	grp := cmdCtx.Session.Group

	if grp == nil {
		return NewUserError("You are not in a group.")
	}
	if grp.LeaderId != actorId {
		return NewUserError("Only the group leader can remove members.")
	}

	// Targeting yourself disbands the whole group.
	if targetId == actorId {
		f.disband(grp)
		return nil
	}

	if !grp.HasMember(targetId) {
		return NewUserError(fmt.Sprintf("%s is not in your group.", target.Player.Name))
	}

	targetPs := f.players.GetPlayer(targetId)
	f.removeMember(cmdCtx.Actor.Name, actorId, targetId, target.Player.Name, targetPs, grp)

	if soloLeader(grp) {
		f.disband(grp)
	}
	return nil
}
