package commands

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/shared"
)

// CombatManager provides combat operations needed by command handlers.
type CombatManager interface {
	StartCombat(attacker, target shared.Actor) error
	AddThreat(source, target shared.Actor, amount int)
	SetThreat(source, target shared.Actor, amount int)
	TopThreat(source, target shared.Actor)
	NotifyHeal(healer, target shared.Actor, amount int)
}

// AssistActor provides the character state needed by the assist handler.
// Embeds shared.Actor because the actor is passed to StartCombat.
type AssistActor interface {
	shared.Actor
	GetFollowingId() string
}

var _ AssistActor = (*game.CharacterInstance)(nil)

// AssistedPlayer provides the state the assist handler reads from the player
// being assisted. This narrow interface lets tests mock the assisted player
// without constructing a full CharacterInstance.
type AssistedPlayer interface {
	Name() string
	CombatTargetId() string
	Location() (zoneId, roomId string)
}

var _ AssistedPlayer = (*game.CharacterInstance)(nil)

// AssistPlayerLookup finds players that can be assisted.
type AssistPlayerLookup interface {
	GetPlayer(charId string) AssistedPlayer
}

// assistPlayerAdapter wraps a PlayerLookup to satisfy AssistPlayerLookup.
type assistPlayerAdapter struct {
	inner PlayerLookup
}

func (a *assistPlayerAdapter) GetPlayer(charId string) AssistedPlayer {
	p := a.inner.GetPlayer(charId)
	if p == nil {
		return nil
	}
	return p
}

// AssistHandlerFactory creates handlers for the assist command.
// When a target player is resolved, the actor joins that player's fight.
// When no target is given, the actor assists their follow leader.
type AssistHandlerFactory struct {
	combat  CombatManager
	zones   ZoneLocator
	players AssistPlayerLookup
	pub     game.Publisher
}

// NewAssistHandlerFactory creates a handler factory for the assist command.
func NewAssistHandlerFactory(combat CombatManager, zones ZoneLocator, players PlayerLookup, pub game.Publisher) *AssistHandlerFactory {
	return &AssistHandlerFactory{combat: combat, zones: zones, players: &assistPlayerAdapter{inner: players}, pub: pub}
}

// Spec returns the optional target player requirement for the assist handler.
func (f *AssistHandlerFactory) Spec() *HandlerSpec {
	return &HandlerSpec{
		Targets: []TargetRequirement{
			{Name: "target", Type: targetTypePlayer, Required: false},
		},
	}
}

// ValidateConfig performs custom validation on the command config.
func (f *AssistHandlerFactory) ValidateConfig(config map[string]string) error {
	return nil
}

// Create returns a compiled CommandFunc for this handler.
func (f *AssistHandlerFactory) Create() (CommandFunc, error) {
	return Adapt[AssistActor](f.handle), nil
}

func (f *AssistHandlerFactory) handle(ctx context.Context, char AssistActor, in *CommandInput) error {
	if char.IsInCombat() {
		return NewUserError("You're already fighting!")
	}
	if char.HasGrant(assets.PerkGrantPeaceful, "") {
		return errPeacefulArea
	}

	assistedId, assistedName, err := f.resolveAssisted(char, in)
	if err != nil {
		return err
	}

	assisted := f.players.GetPlayer(assistedId)
	if assisted == nil {
		return NewUserError(fmt.Sprintf("%s isn't here.", assistedName))
	}

	targetMobId := assisted.CombatTargetId()
	if targetMobId == "" {
		return NewUserError(fmt.Sprintf("%s isn't fighting anyone.", assistedName))
	}

	assistedZone, assistedRoom := assisted.Location()
	targetMob := f.zones.GetZone(assistedZone).GetRoom(assistedRoom).GetMob(targetMobId)
	if err := f.combat.StartCombat(char, targetMob); err != nil {
		return NewUserError(fmt.Sprintf("%s isn't fighting anything you can assist with.", assistedName))
	}

	actorId := char.Id()

	if err := f.pub.Publish(game.SinglePlayer(actorId), nil,
		[]byte(fmt.Sprintf("You jump to %s's aid!", assistedName))); err != nil {
		slog.Warn("failed to notify actor of assist", "error", err)
	}
	if err := f.pub.Publish(game.SinglePlayer(assistedId), nil,
		[]byte(fmt.Sprintf("%s jumps to your aid!", char.Name()))); err != nil {
		slog.Warn("failed to notify assisted player", "error", err)
	}

	zoneID, roomID := char.Location()
	room := f.zones.GetZone(zoneID).GetRoom(roomID)
	roomMsg := fmt.Sprintf("%s jumps to %s's aid!", char.Name(), assistedName)
	if err := f.pub.Publish(room, []string{actorId, assistedId}, []byte(roomMsg)); err != nil {
		slog.Warn("failed to publish room assist message", "error", err)
	}

	return nil
}

// resolveAssisted determines who the actor wants to assist.
// Returns the assisted player's charId and display name.
func (f *AssistHandlerFactory) resolveAssisted(char AssistActor, in *CommandInput) (string, string, error) {
	if target := in.Targets["target"]; target != nil {
		return target.Actor.CharId, target.Actor.Name, nil
	}

	leaderId := char.GetFollowingId()
	leader := f.players.GetPlayer(leaderId)
	if leader == nil {
		return "", "", NewUserError("Assist whom?")
	}

	return leaderId, leader.Name(), nil
}
