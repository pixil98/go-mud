package game

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/pixil98/go-mud/internal/combat"
	"github.com/pixil98/go-mud/internal/storage"
)

// CombatEventHandler implements combat.EventHandler with game-level death logic.
type CombatEventHandler struct {
	world       *WorldState
	pub         Publisher
	defaultZone storage.Identifier
	defaultRoom storage.Identifier
}

func NewCombatEventHandler(world *WorldState, pub Publisher, defaultZone, defaultRoom storage.Identifier) *CombatEventHandler {
	return &CombatEventHandler{
		world:       world,
		pub:         pub,
		defaultZone: defaultZone,
		defaultRoom: defaultRoom,
	}
}

func (h *CombatEventHandler) OnDeath(victim combat.Combatant, zoneID, roomID string) {
	switch v := victim.(type) {
	case *MobCombatant:
		h.onMobDeath(v, zoneID, roomID)
	case *PlayerCombatant:
		h.onPlayerDeath(v, zoneID, roomID)
	}
}

func (h *CombatEventHandler) onMobDeath(mob *MobCombatant, zoneID, roomID string) {
	mi := mob.Instance
	def := mi.Mobile.Get()
	zone := storage.Identifier(zoneID)
	room := storage.Identifier(roomID)

	// Broadcast death message
	msg := fmt.Sprintf("%s is dead! R.I.P.", def.ShortDesc)
	_ = h.pub.PublishToRoom(zone, room, []byte(msg))

	// Create corpse object definition
	corpseAliases := append([]string{"corpse"}, def.Aliases...)
	corpseDef := &Object{
		Aliases:   corpseAliases,
		ShortDesc: fmt.Sprintf("the corpse of %s", def.ShortDesc),
		LongDesc:  fmt.Sprintf("The corpse of %s lies here.", def.ShortDesc),
		Flags:     []string{"container", "immobile"},
	}

	corpseId := fmt.Sprintf("corpse-%s", mi.InstanceId)
	corpseSmartId := storage.NewResolvedSmartIdentifier[*Object](corpseId, corpseDef)

	corpse := &ObjectInstance{
		InstanceId: uuid.New().String(),
		Object:     corpseSmartId,
		Contents:   NewInventory(),
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
	ri := h.world.Instances()[zone].GetRoom(room)
	if ri != nil {
		ri.AddObj(corpse)
		ri.RemoveMob(mi.InstanceId)
	}
}

func (h *CombatEventHandler) onPlayerDeath(pc *PlayerCombatant, zoneID, roomID string) {
	zone := storage.Identifier(zoneID)
	room := storage.Identifier(roomID)

	// Broadcast death message to room
	msg := fmt.Sprintf("%s has been slain!", pc.Character.Name)
	_ = h.pub.PublishToRoom(zone, room, []byte(msg))

	// Send personal message
	_ = h.pub.PublishToPlayer(pc.CharId, []byte("You have been slain! You awaken in a familiar place..."))

	// Restore HP
	pc.Character.CurrentHP = pc.Character.MaxHP

	// Move to default spawn
	fromRoom := h.world.Instances()[zone].GetRoom(room)
	toRoom := h.world.Instances()[h.defaultZone].GetRoom(h.defaultRoom)
	if fromRoom != nil && toRoom != nil {
		pc.Player.Move(fromRoom, toRoom)

		// Show new room
		roomDesc := toRoom.Describe(pc.Character.Name)
		_ = h.pub.PublishToPlayer(pc.CharId, []byte(roomDesc))
	}
}

// CombatMessagePublisher adapts game.Publisher to combat.MessagePublisher.
type CombatMessagePublisher struct {
	pub Publisher
}

func NewCombatMessagePublisher(pub Publisher) *CombatMessagePublisher {
	return &CombatMessagePublisher{pub: pub}
}

func (p *CombatMessagePublisher) SendToRoom(zoneID, roomID string, msg string) {
	_ = p.pub.PublishToRoom(storage.Identifier(zoneID), storage.Identifier(roomID), []byte(msg))
}

func (p *CombatMessagePublisher) SendToPlayer(id string, msg string) {
	_ = p.pub.PublishToPlayer(storage.Identifier(id), []byte(msg))
}
