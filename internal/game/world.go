package game

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pixil98/go-mud/internal/storage"
)

// WorldState is the single source of truth for all mutable game state.
// All access must go through its methods to ensure thread-safety.
type WorldState struct {
	mu         sync.RWMutex
	subscriber Subscriber
	players    map[storage.Identifier]*PlayerState

	instances map[storage.Identifier]*ZoneInstance
}

// NewWorldState creates a new WorldState with zone and room instances initialized.
func NewWorldState(sub Subscriber, zones storage.Storer[*Zone], rooms storage.Storer[*Room]) (*WorldState, error) {
	// Build zone instances
	instances := make(map[storage.Identifier]*ZoneInstance)
	for zoneId, zone := range zones.GetAll() {
		zi, err := NewZoneInstance(storage.NewResolvedSmartIdentifier(string(zoneId), zone))
		if err != nil {
			return nil, err
		}
		instances[zoneId] = zi
	}

	// Build room instances and add to their zones
	for roomId, room := range rooms.GetAll() {
		zoneId := storage.Identifier(room.Zone.Id())
		if zi, ok := instances[zoneId]; ok {
			ri, err := NewRoomInstance(storage.NewResolvedSmartIdentifier(string(roomId), room))
			if err != nil {
				return nil, err
			}
			zi.AddRoom(ri)
		}
	}

	return &WorldState{
		subscriber: sub,
		players:    make(map[storage.Identifier]*PlayerState),
		instances:  instances,
	}, nil
}

// Instances returns all zone instances.
func (w *WorldState) Instances() map[storage.Identifier]*ZoneInstance {
	return w.instances
}

// GetPlayer returns the player state. Returns nil if player not found.
func (w *WorldState) GetPlayer(charId storage.Identifier) *PlayerState {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.players[charId]
}

// AddPlayer registers a new player in the world state and adds them to the room instance.
func (w *WorldState) AddPlayer(charId storage.Identifier, char *Character, msgs chan []byte, zoneId storage.Identifier, roomId storage.Identifier) error {
	w.mu.Lock()
	if _, exists := w.players[charId]; exists {
		w.mu.Unlock()
		return ErrPlayerExists
	}

	ps := &PlayerState{
		subscriber:   w.subscriber,
		subs:         make(map[string]func()),
		msgs:         msgs,
		CharId:       charId,
		Character:    char,
		ZoneId:       zoneId,
		RoomId:       roomId,
		Quit:         false,
		LastActivity: time.Now(),
		done:         make(chan struct{}),
	}
	w.players[charId] = ps
	room := w.instances[zoneId].GetRoom(roomId)
	w.mu.Unlock()

	room.AddPlayer(charId, ps)
	return nil
}

// RemovePlayer removes a player from the world state and from the room instance.
func (w *WorldState) RemovePlayer(charId storage.Identifier) error {
	w.mu.Lock()
	ps, exists := w.players[charId]
	if !exists {
		w.mu.Unlock()
		return ErrPlayerNotFound
	}

	room := w.instances[ps.ZoneId].GetRoom(ps.RoomId)
	delete(w.players, charId)
	w.mu.Unlock()

	room.RemovePlayer(charId)
	return nil
}

// SetPlayerQuit sets the quit flag for a player.
func (w *WorldState) SetPlayerQuit(charId storage.Identifier, quit bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	p, exists := w.players[charId]
	if !exists {
		return ErrPlayerNotFound
	}

	p.Quit = quit
	return nil
}

// MarkPlayerActive resets the player's idle timer.
func (w *WorldState) MarkPlayerActive(charId storage.Identifier) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if p, ok := w.players[charId]; ok {
		p.LastActivity = time.Now()
	}
}

// ForEachPlayer calls fn for each player in the world while holding the lock.
func (w *WorldState) ForEachPlayer(fn func(storage.Identifier, *PlayerState)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for id, ps := range w.players {
		fn(id, ps)
	}
}

// Subscriber provides the ability to subscribe to message subjects
type Subscriber interface {
	Subscribe(subject string, handler func(data []byte)) (unsubscribe func(), err error)
}

// Tick processes zone resets based on their reset mode and lifespan.
func (w *WorldState) Tick(ctx context.Context) error {
	for _, zi := range w.instances {
		err := zi.Reset(false, w.instances)
		if err != nil {
			return err
		}
	}

	// Regenerate out-of-combat entities.
	w.ForEachPlayer(func(_ storage.Identifier, ps *PlayerState) {
		if !ps.InCombat && ps.Character.CurrentHP < ps.Character.MaxHP {
			ps.Character.Regenerate(1)
		}
	})
	for _, zi := range w.instances {
		for _, ri := range zi.rooms {
			ri.mu.RLock()
			for _, mi := range ri.mobiles {
				if !mi.InCombat && mi.CurrentHP < mi.MaxHP {
					mi.Regenerate(1)
				}
			}
			ri.mu.RUnlock()
		}
	}

	return nil
}

// PlayerState holds all mutable state for an active player.
type PlayerState struct {
	subscriber Subscriber
	msgs       chan []byte

	CharId    storage.Identifier
	Character *Character

	// Location
	ZoneId storage.Identifier
	RoomId storage.Identifier

	// Subscriptions
	subs map[string]func()

	// Session state
	Quit         bool
	InCombat     bool
	LastActivity time.Time

	// Connection management: closed to signal the active Play() goroutine to exit.
	done chan struct{}

	// Linkless state: player's connection dropped but they remain in the world.
	Linkless   bool
	LinklessAt time.Time
}

// Flags returns display labels for the player's current state (e.g., "linkless").
func (p *PlayerState) Flags() []string {
	var flags []string
	if p.InCombat {
		flags = append(flags, "fighting")
	}
	if p.Linkless {
		flags = append(flags, "linkless")
	}
	return flags
}

// Done returns the channel that is closed when this session is evicted by a reconnection.
func (p *PlayerState) Done() <-chan struct{} {
	return p.done
}

// Location returns the player's current zone and room.
func (p *PlayerState) Location() (zoneId, roomId storage.Identifier) {
	return p.ZoneId, p.RoomId
}

// Move updates the player's location and room instance player lists.
func (p *PlayerState) Move(fromRoom, toRoom *RoomInstance) {
	toZoneId := storage.Identifier(toRoom.Room.Get().Zone.Id())
	toRoomId := storage.Identifier(toRoom.Room.Id())

	fromRoom.RemovePlayer(p.CharId)
	toRoom.AddPlayer(p.CharId, p)

	p.ZoneId = toZoneId
	p.RoomId = toRoomId
}

// Subscribe adds a new subscription
func (p *PlayerState) Subscribe(subject string) error {
	if p.subscriber == nil {
		return fmt.Errorf("subscriber is nil")
	}

	unsub, err := p.subscriber.Subscribe(subject, func(data []byte) {
		p.msgs <- data
	})

	// If we some how are subscribing to a channel we already think we have
	// unsubscribe from the existing one.
	if unsub, ok := p.subs[subject]; ok {
		unsub()
	}

	if err != nil {
		return fmt.Errorf("subscribing to channel '%s': %w", subject, err)
	}
	p.subs[subject] = unsub
	return nil

}

// Unsubscribe removes a subscription by name
func (p *PlayerState) Unsubscribe(subject string) {
	if unsub, ok := p.subs[subject]; ok {
		unsub()
		delete(p.subs, subject)
	}
}

// UnsubscribeAll removes all subscriptions
func (p *PlayerState) UnsubscribeAll() {
	for name, unsub := range p.subs {
		unsub()
		delete(p.subs, name)
	}
}

// Kick closes the done channel, signaling the active Play() goroutine to exit.
// It is safe to call multiple times; subsequent calls are no-ops.
func (p *PlayerState) Kick() {
	select {
	case <-p.done:
		// already closed
	default:
		close(p.done)
	}
}

// Reattach swaps the msgs channel and done channel for a reconnecting player.
// It unsubscribes all old NATS subscriptions (their closures reference the old msgs channel),
// clears the linkless flag, and creates a fresh done channel.
// The caller is responsible for re-subscribing to NATS channels after this call.
func (p *PlayerState) Reattach(msgs chan []byte) {
	p.UnsubscribeAll()
	p.msgs = msgs
	p.done = make(chan struct{})
	p.Linkless = false
	p.LinklessAt = time.Time{}
	p.LastActivity = time.Now()
}

// SaveCharacter persists the character's current session state (location, inventory, etc.)
// to the character store.
func (ps *PlayerState) SaveCharacter(chars storage.Storer[*Character]) error {
	ps.Character.LastZone = ps.ZoneId
	ps.Character.LastRoom = ps.RoomId
	return chars.Save(string(ps.CharId), ps.Character)
}

// MarkLinkless sets the player as linkless and unsubscribes all NATS subscriptions
// to prevent channel fill-up while they have no active connection.
func (p *PlayerState) MarkLinkless() {
	p.Linkless = true
	p.LinklessAt = time.Now()
	p.UnsubscribeAll()
}
