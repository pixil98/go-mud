package combat

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/storage"
)

// CombatEventHandler handles combat events that require game-level death logic.
type CombatEventHandler struct {
	world       *game.WorldState
	pub         game.Publisher
	defaultZone string
	defaultRoom string
}

func NewCombatEventHandler(world *game.WorldState, pub game.Publisher, defaultZone, defaultRoom string) *CombatEventHandler {
	return &CombatEventHandler{
		world:       world,
		pub:         pub,
		defaultZone: defaultZone,
		defaultRoom: defaultRoom,
	}
}

func (h *CombatEventHandler) OnDeath(dctx DeathContext) {
	switch v := dctx.Victim.(type) {
	case *MobCombatant:
		h.onMobDeath(v, dctx)
	case *PlayerCombatant:
		h.onPlayerDeath(v, dctx)
	}
}

func (h *CombatEventHandler) onMobDeath(mob *MobCombatant, dctx DeathContext) {
	mi := mob.Instance
	def := mi.Mobile.Get()

	// Create corpse object definition
	corpseAliases := append([]string{"corpse"}, def.Aliases...)
	corpseDef := &game.Object{
		Aliases:      corpseAliases,
		ShortDesc:    fmt.Sprintf("the corpse of %s", def.ShortDesc),
		LongDesc:     fmt.Sprintf("The corpse of %s lies here.", def.ShortDesc),
		DetailedDesc: fmt.Sprintf("The lifeless remains of %s lie here, growing cold.", def.ShortDesc),
		Flags:        []string{"container", "immobile"},
	}

	corpseId := fmt.Sprintf("corpse-%s", mi.InstanceId)
	corpseSmartId := storage.NewResolvedSmartIdentifier[*game.Object](corpseId, corpseDef)

	corpse := &game.ObjectInstance{
		InstanceId: uuid.New().String(),
		Object:     corpseSmartId,
		Contents:   game.NewInventory(),
	}

	// Transfer inventory to corpse
	for id, obj := range mi.Inventory.Objs {
		mi.Inventory.RemoveObj(id)
		corpse.Contents.AddObj(obj)
	}

	// Transfer equipment to corpse
	for _, slot := range mi.Equipment.Objs {
		if slot.Obj != nil {
			mi.Equipment.RemoveObj(slot.Obj.InstanceId)
			corpse.Contents.AddObj(slot.Obj)
		}
	}

	// Place corpse in room and remove mob
	ri := h.world.Instances()[dctx.ZoneID].GetRoom(dctx.RoomID)
	if ri != nil {
		ri.AddObj(corpse)
		ri.RemoveMob(mi.InstanceId)
	}
}

func (h *CombatEventHandler) onPlayerDeath(pc *PlayerCombatant, dctx DeathContext) {
	deadId := string(pc.Character.Id())
	char := pc.Character.Get()

	// Clear following on both sides: the dead player stops following,
	// and anyone following the dead player stops too.
	pc.Player.FollowingId = ""
	fromRoom := h.world.Instances()[dctx.ZoneID].GetRoom(dctx.RoomID)
	if fromRoom != nil {
		fromRoom.ForEachPlayer(func(charId string, ps *game.PlayerState) {
			if ps.FollowingId == deadId {
				ps.FollowingId = ""
				_ = h.pub.Publish(game.SinglePlayer(charId), nil,
					[]byte(fmt.Sprintf("You stop following %s.", char.Name)))
			}
		})
	}

	// Send personal message
	_ = h.pub.Publish(game.SinglePlayer(pc.Character.Id()), nil, []byte("You have been slain! You awaken in a familiar place..."))

	// Restore HP
	char.CurrentHP = char.MaxHP

	// Move to default spawn
	toRoom := h.world.Instances()[h.defaultZone].GetRoom(h.defaultRoom)
	if fromRoom != nil && toRoom != nil {
		pc.Player.Move(fromRoom, toRoom)

		// Show new room
		roomDesc := toRoom.Describe(char.Name)
		_ = h.pub.Publish(game.SinglePlayer(pc.Character.Id()), nil, []byte(roomDesc))
	}
}
