package game

import (
	"context"
	"math/rand/v2"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

// newTestObj creates a minimal ObjectInstance for testing. An optional lifetime
// (in ticks) can be provided; zero or omitted means permanent.
func newTestObj(id string, lifetime ...int) *ObjectInstance {
	lt := 0
	if len(lifetime) > 0 {
		lt = lifetime[0]
	}
	obj := storage.NewResolvedSmartIdentifier(id, &assets.Object{
		Aliases:   []string{id},
		ShortDesc: id,
		Lifetime:  lt,
	})
	oi, _ := NewObjectInstance(obj)
	return oi
}

// newTestZone creates a minimal ZoneInstance for testing.
func newTestZone(id string) *ZoneInstance {
	zi, _ := NewZoneInstance(
		storage.NewResolvedSmartIdentifier(id, &assets.Zone{ResetMode: assets.ZoneResetNever}),
		nil,
	)
	return zi
}

// fakeSubscriber is a test double for the Subscriber interface.
type fakeSubscriber struct {
	subs map[string]func([]byte)
}

func (fs *fakeSubscriber) Subscribe(subject string, handler func(data []byte)) (func(), error) {
	if fs.subs == nil {
		fs.subs = make(map[string]func([]byte))
	}
	fs.subs[subject] = handler
	return func() { delete(fs.subs, subject) }, nil
}

// fakeCommander is a test double for the Commander interface that records calls.
type fakeCommander struct {
	commands  []string
	abilities []string
}

func (fc *fakeCommander) ExecCommand(_ context.Context, cmd string, _ ...string) error {
	fc.commands = append(fc.commands, cmd)
	return nil
}

func (fc *fakeCommander) ExecAbility(_ context.Context, id string, _ Actor) error {
	fc.abilities = append(fc.abilities, id)
	return nil
}

// neverRand always returns 1 so probability-gated actions (wander, scavenge) are skipped.
func neverRand(_ int) int { return 1 }

// zeroRand always returns 0, forcing probability-gated actions to trigger.
func zeroRand(_ int) int { return 0 }

// newTestRoom creates a minimal RoomInstance for testing.
func newTestRoom(id string) *RoomInstance {
	ri, _ := NewRoomInstance(storage.NewResolvedSmartIdentifier(id, &assets.Room{Name: id}))
	return ri
}

// newTestCI creates a minimal CharacterInstance for follow tree and actor tests.
func newTestCI(id, name string) *CharacterInstance {
	charRef := storage.NewResolvedSmartIdentifier(id, &assets.Character{Name: name})
	ci := &CharacterInstance{
		Character: charRef,
		ActorInstance: ActorInstance{
			InstanceId: id,
			PerkCache:  *NewPerkCache(nil, nil),
		},
	}
	ci.self = ci
	return ci
}

// fakeStore is an in-memory Storer for use in tests.
type fakeStore[T storage.ValidatingSpec] struct {
	records map[string]T
	saved   map[string]T
}

func (fs *fakeStore[T]) Save(id string, v T) error {
	if fs.saved == nil {
		fs.saved = make(map[string]T)
	}
	fs.saved[id] = v
	return nil
}

func (fs *fakeStore[T]) Get(id string) T {
	return fs.records[id]
}

func (fs *fakeStore[T]) GetAll() map[string]T {
	if fs.records == nil {
		return make(map[string]T)
	}
	return fs.records
}

// newFakeStore creates a fakeStore pre-populated with the given records.
func newFakeStore[T storage.ValidatingSpec](records map[string]T) *fakeStore[T] {
	if records == nil {
		records = make(map[string]T)
	}
	return &fakeStore[T]{records: records}
}

// newTestWorld builds a minimal WorldState with one zone ("z1") and one room ("r1").
// The world has no commander factory; callers that need AddPlayer should call
// w.SetCommanderFactory before adding players.
func newTestWorld() (*WorldState, *ZoneInstance, *RoomInstance) {
	zone := &assets.Zone{ResetMode: assets.ZoneResetNever}
	room := &assets.Room{
		Name: "Test Room",
		Zone: storage.NewResolvedSmartIdentifier("z1", zone),
	}
	zones := newFakeStore[*assets.Zone](map[string]*assets.Zone{"z1": zone})
	rooms := newFakeStore[*assets.Room](map[string]*assets.Room{"r1": room})

	w, err := NewWorldState(&fakeSubscriber{}, zones, rooms)
	if err != nil {
		panic("newTestWorld: " + err.Error())
	}
	w.SetCommanderFactory(func(Actor) Commander { return &fakeCommander{} })

	zi := w.GetZone("z1")
	ri := zi.GetRoom("r1")
	return w, zi, ri
}

// newTestMI creates a minimal MobileInstance with a deterministic InstanceId.
// randIntN defaults to rand.IntN; tests can override it for deterministic behavior.
func newTestMI(id, name string) *MobileInstance {
	mobRef := storage.NewResolvedSmartIdentifier(id, &assets.Mobile{ShortDesc: name})
	eq := NewEquipment()
	mi := &MobileInstance{
		Mobile:   mobRef,
		randIntN: rand.IntN,
		ActorInstance: ActorInstance{
			InstanceId: id,
			PerkCache:  *NewPerkCache(nil, map[string]PerkSource{"equipment": eq}),
			inventory:  NewInventory(),
			equipment:  eq,
		},
	}
	mi.self = mi
	return mi
}
