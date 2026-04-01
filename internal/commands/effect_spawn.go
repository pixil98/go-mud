package commands

import (
	"fmt"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/game"
	"github.com/pixil98/go-mud/internal/shared"
	"github.com/pixil98/go-mud/internal/storage"
)

// Spawn destination constants for spawnObjEffect.
const (
	SpawnDestRoom      = "room"
	SpawnDestInventory = "inventory"
)

// spawnMobEffect spawns a mobile instance into the caster's room.
//
// Config fields:
//   - "mobile_id" (string, required): the asset ID of the mobile to spawn.
type spawnMobEffect struct {
	mobiles storage.Storer[*assets.Mobile]
	world   ZoneLocator
}

func (e *spawnMobEffect) Spec() *HandlerSpec { return nil }

func (e *spawnMobEffect) ValidateConfig(config map[string]string) error {
	mobId := config["mobile_id"]
	if mobId == "" {
		return fmt.Errorf("mobile_id config required")
	}
	if e.mobiles.Get(mobId) == nil {
		return fmt.Errorf("mobile %q not found", mobId)
	}
	return nil
}

func (e *spawnMobEffect) Create(_ string, config map[string]string, _ []assets.TargetSpec) EffectFunc {
	mobId := config["mobile_id"]

	return func(actor shared.Actor, _ map[string]*TargetRef, _ *AbilityResult) error {
		si := storage.NewSmartIdentifier[*assets.Mobile](mobId)
		if err := si.Resolve(e.mobiles); err != nil {
			return fmt.Errorf("spawn_mob: %w", err)
		}
		mi, err := game.NewMobileInstance(si)
		if err != nil {
			return fmt.Errorf("spawn_mob: %w", err)
		}

		zoneId, roomId := actor.Location()
		zi := e.world.GetZone(zoneId)
		if zi == nil {
			return nil
		}
		ri := zi.GetRoom(roomId)
		if ri == nil {
			return nil
		}
		ri.AddMob(mi)
		return nil
	}
}

// spawnObjEffect spawns an object into the caster's room or inventory.
//
// Config fields:
//   - "object_id" (string, required): the asset ID of the object to spawn.
//   - "destination" (string, optional): "room" (default) or "inventory".
type spawnObjEffect struct {
	objects storage.Storer[*assets.Object]
	world   ZoneLocator
}

func (e *spawnObjEffect) Spec() *HandlerSpec { return nil }

func (e *spawnObjEffect) ValidateConfig(config map[string]string) error {
	objId := config["object_id"]
	if objId == "" {
		return fmt.Errorf("object_id config required")
	}
	if e.objects.Get(objId) == nil {
		return fmt.Errorf("object %q not found", objId)
	}
	dest := config["destination"]
	if dest != "" && dest != SpawnDestRoom && dest != SpawnDestInventory {
		return fmt.Errorf("destination must be %q or %q", SpawnDestRoom, SpawnDestInventory)
	}
	return nil
}

func (e *spawnObjEffect) Create(_ string, config map[string]string, _ []assets.TargetSpec) EffectFunc {
	objId := config["object_id"]
	dest := config["destination"]
	if dest == "" {
		dest = SpawnDestRoom
	}

	return func(actor shared.Actor, _ map[string]*TargetRef, _ *AbilityResult) error {
		si := storage.NewSmartIdentifier[*assets.Object](objId)
		if err := si.Resolve(e.objects); err != nil {
			return fmt.Errorf("spawn_obj: %w", err)
		}
		oi, err := game.NewObjectInstance(si)
		if err != nil {
			return fmt.Errorf("spawn_obj: %w", err)
		}
		oi.ActivateDecay()

		switch dest {
		case SpawnDestInventory:
			actor.Inventory().AddObj(oi)
		case SpawnDestRoom:
			zoneId, roomId := actor.Location()
			zi := e.world.GetZone(zoneId)
			if zi == nil {
				return nil
			}
			ri := zi.GetRoom(roomId)
			if ri == nil {
				return nil
			}
			ri.AddObj(oi)
		}
		return nil
	}
}
