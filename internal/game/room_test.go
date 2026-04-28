package game

import (
	"strings"
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

func TestResolvedExit_ClosedLocked(t *testing.T) {
	tests := map[string]struct {
		setClosed bool
		setLocked bool
	}{
		"closed and locked":         {setClosed: true, setLocked: true},
		"closed only":               {setClosed: true, setLocked: false},
		"locked only":               {setClosed: false, setLocked: true},
		"neither closed nor locked": {setClosed: false, setLocked: false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			re := &ResolvedExit{}
			re.SetClosed(tc.setClosed)
			re.SetLocked(tc.setLocked)
			if got := re.IsClosed(); got != tc.setClosed {
				t.Errorf("IsClosed() = %v, want %v", got, tc.setClosed)
			}
			if got := re.IsLocked(); got != tc.setLocked {
				t.Errorf("IsLocked() = %v, want %v", got, tc.setLocked)
			}
		})
	}
}

func TestResolvedExit_OtherSide(t *testing.T) {
	tests := map[string]struct {
		setupDest func(source, dest, other *RoomInstance) *ResolvedExit
		wantDir   string
		wantNil   bool
	}{
		"nil Dest returns empty": {
			setupDest: func(_, _, _ *RoomInstance) *ResolvedExit {
				return &ResolvedExit{}
			},
			wantNil: true,
		},
		"dest has no exits returns empty": {
			setupDest: func(_, dest, _ *RoomInstance) *ResolvedExit {
				return &ResolvedExit{Dest: dest}
			},
			wantNil: true,
		},
		"dest exit without closure is skipped": {
			setupDest: func(source, dest, _ *RoomInstance) *ResolvedExit {
				dest.exits["south"] = &ResolvedExit{Exit: assets.Exit{}, Dest: source}
				return &ResolvedExit{Dest: dest}
			},
			wantNil: true,
		},
		"dest exit leading to unrelated room is skipped": {
			setupDest: func(_, dest, unrelated *RoomInstance) *ResolvedExit {
				dest.exits["south"] = &ResolvedExit{
					Exit: assets.Exit{Closure: &assets.Closure{Name: "gate"}},
					Dest: unrelated,
				}
				return &ResolvedExit{Dest: dest}
			},
			wantNil: true,
		},
		"reverse exit with closure found": {
			setupDest: func(source, dest, _ *RoomInstance) *ResolvedExit {
				dest.exits["south"] = &ResolvedExit{
					Exit: assets.Exit{Closure: &assets.Closure{Name: "gate"}},
					Dest: source,
				}
				return &ResolvedExit{Dest: dest}
			},
			wantDir: "south",
			wantNil: false,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			source := newTestRoom("source")
			dest := newTestRoom("dest")
			unrelated := newTestRoom("unrelated")
			re := tc.setupDest(source, dest, unrelated)

			dir, other := re.OtherSide(source)
			if tc.wantNil {
				if dir != "" || other != nil {
					t.Errorf("OtherSide() = (%q, %v), want (\"\", nil)", dir, other)
				}
				return
			}
			if dir != tc.wantDir {
				t.Errorf("OtherSide() dir = %q, want %q", dir, tc.wantDir)
			}
			if other == nil {
				t.Error("OtherSide() exit = nil, want non-nil")
			}
		})
	}
}

func TestRoomInstance_FindExit(t *testing.T) {
	tests := map[string]struct {
		query   string
		wantDir string
		wantNil bool
	}{
		"find by direction key":              {query: "north", wantDir: "north"},
		"find by direction case-insensitive": {query: "NORTH", wantDir: "north"},
		"find by closure name":               {query: "gate", wantDir: "south"},
		"find by closure name mixed case":    {query: "Gate", wantDir: "south"},
		"no match returns empty":             {query: "east", wantNil: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ri := newTestRoom("r")
			ri.exits["north"] = &ResolvedExit{Exit: assets.Exit{}}
			ri.exits["south"] = &ResolvedExit{Exit: assets.Exit{Closure: &assets.Closure{Name: "gate"}}}

			dir, re := ri.FindExit(tc.query)
			if tc.wantNil {
				if dir != "" || re != nil {
					t.Errorf("FindExit(%q) = (%q, %v), want (\"\", nil)", tc.query, dir, re)
				}
				return
			}
			if dir != tc.wantDir {
				t.Errorf("FindExit(%q) dir = %q, want %q", tc.query, dir, tc.wantDir)
			}
			if re == nil {
				t.Errorf("FindExit(%q) exit = nil, want non-nil", tc.query)
			}
		})
	}
}

func TestRoomInstance_MobCRUD(t *testing.T) {
	tests := map[string]struct {
		setup func(*RoomInstance) string
		check func(*testing.T, *RoomInstance, string)
	}{
		"AddMob makes mob findable": {
			setup: func(ri *RoomInstance) string {
				mi := newTestMI("m1", "mob one")
				ri.AddMob(mi)
				return mi.Id()
			},
			check: func(t *testing.T, ri *RoomInstance, id string) {
				if ri.GetMob(id) == nil {
					t.Error("GetMob should find added mob")
				}
			},
		},
		"RemoveMob returns removed instance and clears it": {
			setup: func(ri *RoomInstance) string {
				mi := newTestMI("m1", "mob one")
				ri.AddMob(mi)
				return mi.Id()
			},
			check: func(t *testing.T, ri *RoomInstance, id string) {
				got := ri.RemoveMob(id)
				if got == nil {
					t.Error("RemoveMob returned nil, want mob")
				}
				if ri.GetMob(id) != nil {
					t.Error("mob should not be findable after RemoveMob")
				}
			},
		},
		"RemoveMob of missing id returns nil": {
			setup: func(*RoomInstance) string { return "missing" },
			check: func(t *testing.T, ri *RoomInstance, id string) {
				if got := ri.RemoveMob(id); got != nil {
					t.Errorf("RemoveMob(missing) = %v, want nil", got)
				}
			},
		},
		"GetMob returns nil for unknown id": {
			setup: func(*RoomInstance) string { return "unknown" },
			check: func(t *testing.T, ri *RoomInstance, id string) {
				if got := ri.GetMob(id); got != nil {
					t.Errorf("GetMob(unknown) = %v, want nil", got)
				}
			},
		},
		"FindMobs filters by matcher": {
			setup: func(ri *RoomInstance) string {
				m1 := newTestMI("m1", "mob one")
				m1.level = 1
				m5 := newTestMI("m5", "mob five")
				m5.level = 5
				ri.AddMob(m1)
				ri.AddMob(m5)
				return ""
			},
			check: func(t *testing.T, ri *RoomInstance, _ string) {
				got := ri.FindMobs(func(mi *MobileInstance) bool { return mi.Level() == 5 })
				if len(got) != 1 {
					t.Errorf("FindMobs at level 5: got %d, want 1", len(got))
				}
			},
		},
		"ForEachMob visits all mobs": {
			setup: func(ri *RoomInstance) string {
				ri.AddMob(newTestMI("m1", "mob one"))
				ri.AddMob(newTestMI("m2", "mob two"))
				return ""
			},
			check: func(t *testing.T, ri *RoomInstance, _ string) {
				count := 0
				ri.ForEachMob(func(*MobileInstance) { count++ })
				if count != 2 {
					t.Errorf("ForEachMob visited %d, want 2", count)
				}
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ri := newTestRoom("r")
			id := tc.setup(ri)
			tc.check(t, ri, id)
		})
	}
}

func TestRoomInstance_PlayerCRUD(t *testing.T) {
	tests := map[string]struct {
		setup func(*RoomInstance) string
		check func(*testing.T, *RoomInstance, string)
	}{
		"AddPlayer makes player findable and increments count": {
			setup: func(ri *RoomInstance) string {
				ci := newTestCharacterInstance()
				ri.AddPlayer(ci.Id(), ci)
				return ci.Id()
			},
			check: func(t *testing.T, ri *RoomInstance, id string) {
				found := ri.FindPlayers(func(ci *CharacterInstance) bool { return ci.Id() == id })
				if len(found) != 1 {
					t.Errorf("FindPlayers after AddPlayer: got %d, want 1", len(found))
				}
				if ri.PlayerCount() != 1 {
					t.Errorf("PlayerCount() = %d, want 1", ri.PlayerCount())
				}
			},
		},
		"RemovePlayer decrements count": {
			setup: func(ri *RoomInstance) string {
				ci := newTestCharacterInstance()
				ri.AddPlayer(ci.Id(), ci)
				return ci.Id()
			},
			check: func(t *testing.T, ri *RoomInstance, id string) {
				ri.RemovePlayer(id)
				if ri.PlayerCount() != 0 {
					t.Errorf("PlayerCount() after RemovePlayer = %d, want 0", ri.PlayerCount())
				}
			},
		},
		"ForEachPlayer visits all players": {
			setup: func(ri *RoomInstance) string {
				ri.AddPlayer("a", newTestCharacterInstance())
				ri.AddPlayer("b", newTestCharacterInstance())
				return ""
			},
			check: func(t *testing.T, ri *RoomInstance, _ string) {
				count := 0
				ri.ForEachPlayer(func(string, *CharacterInstance) { count++ })
				if count != 2 {
					t.Errorf("ForEachPlayer visited %d, want 2", count)
				}
			},
		},
		"PlayerCount returns zero for empty room": {
			setup: func(*RoomInstance) string { return "" },
			check: func(t *testing.T, ri *RoomInstance, _ string) {
				if got := ri.PlayerCount(); got != 0 {
					t.Errorf("PlayerCount() = %d, want 0", got)
				}
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ri := newTestRoom("r")
			id := tc.setup(ri)
			tc.check(t, ri, id)
		})
	}
}

func TestRoomInstance_ObjectCRUD(t *testing.T) {
	tests := map[string]struct {
		setup func(*RoomInstance) string
		check func(*testing.T, *RoomInstance, string)
	}{
		"AddObj makes object findable": {
			setup: func(ri *RoomInstance) string {
				oi := newTestObj("sword")
				ri.AddObj(oi)
				return oi.InstanceId
			},
			check: func(t *testing.T, ri *RoomInstance, id string) {
				found := ri.FindObjs(func(oi *ObjectInstance) bool { return oi.InstanceId == id })
				if len(found) != 1 {
					t.Errorf("FindObjs found %d, want 1", len(found))
				}
			},
		},
		"RemoveObj returns removed instance": {
			setup: func(ri *RoomInstance) string {
				oi := newTestObj("sword")
				ri.AddObj(oi)
				return oi.InstanceId
			},
			check: func(t *testing.T, ri *RoomInstance, id string) {
				got := ri.RemoveObj(id)
				if got == nil {
					t.Error("RemoveObj returned nil")
				}
				remaining := ri.FindObjs(func(*ObjectInstance) bool { return true })
				if len(remaining) != 0 {
					t.Errorf("%d objects remain after RemoveObj, want 0", len(remaining))
				}
			},
		},
		"RemoveObj of missing id returns nil": {
			setup: func(*RoomInstance) string { return "missing" },
			check: func(t *testing.T, ri *RoomInstance, id string) {
				if got := ri.RemoveObj(id); got != nil {
					t.Errorf("RemoveObj(missing) = %v, want nil", got)
				}
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ri := newTestRoom("r")
			id := tc.setup(ri)
			tc.check(t, ri, id)
		})
	}
}

func TestRoomInstance_ForEachActor(t *testing.T) {
	tests := map[string]struct {
		mobs    int
		players int
		want    int
	}{
		"empty room visits nobody": {mobs: 0, players: 0, want: 0},
		"mobs only":                {mobs: 2, players: 0, want: 2},
		"players only":             {mobs: 0, players: 2, want: 2},
		"mixed mobs and players":   {mobs: 1, players: 2, want: 3},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ri := newTestRoom("r")
			for i := 0; i < tc.mobs; i++ {
				ri.AddMob(newTestMI(string(rune('a'+i)), "mob"))
			}
			for i := 0; i < tc.players; i++ {
				id := string(rune('A' + i))
				ci := newTestCI(id, "player")
				ri.AddPlayer(id, ci)
			}
			count := 0
			ri.ForEachActor(func(Actor) { count++ })
			if count != tc.want {
				t.Errorf("ForEachActor visited %d, want %d", count, tc.want)
			}
		})
	}
}

func TestRoomInstance_Zone(t *testing.T) {
	tests := map[string]struct {
		addToZone bool
		wantNil   bool
	}{
		"nil before AddRoom": {addToZone: false, wantNil: true},
		"set after AddRoom":  {addToZone: true, wantNil: false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ri := newTestRoom("r")
			if tc.addToZone {
				zi := newTestZone("z")
				zi.AddRoom(ri)
			}
			got := ri.Zone()
			if tc.wantNil {
				if got != nil {
					t.Error("Zone() should be nil before AddRoom")
				}
				return
			}
			if got == nil {
				t.Error("Zone() should not be nil after AddRoom")
			}
		})
	}
}

func TestRoomInstance_Tick(t *testing.T) {
	tests := map[string]struct {
		lifetime  int
		activate  bool
		ticks     int
		wantCount int
	}{
		"permanent object remains": {
			lifetime: 0, ticks: 5, wantCount: 1,
		},
		"expired object removed": {
			lifetime: 2, activate: true, ticks: 2, wantCount: 0,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ri := newTestRoom("r")
			oi := newTestObj("sword", tc.lifetime)
			if tc.activate {
				oi.ActivateDecay()
			}
			ri.AddObj(oi)
			for range tc.ticks {
				ri.Tick()
			}
			found := ri.FindObjs(func(*ObjectInstance) bool { return true })
			if len(found) != tc.wantCount {
				t.Errorf("objects after Tick = %d, want %d", len(found), tc.wantCount)
			}
		})
	}
}

func TestRoomInstance_FindExtraDesc(t *testing.T) {
	roomDef := &assets.Room{
		Name: "test-room",
		ExtraDescs: []assets.ExtraDesc{
			{Keywords: []string{"a", "b"}, Description: "desc-ab"},
			{Keywords: []string{"c"}, Description: "desc-c"},
		},
	}

	objWithDesc := storage.NewResolvedSmartIdentifier("test-obj", &assets.Object{
		Aliases:   []string{"test-obj"},
		ShortDesc: "test-obj",
		ExtraDescs: []assets.ExtraDesc{
			{Keywords: []string{"d"}, Description: "desc-d"},
		},
	})
	oi, err := NewObjectInstance(objWithDesc)
	if err != nil {
		t.Fatalf("spawning object: %v", err)
	}

	ri := &RoomInstance{
		Room:    storage.NewResolvedSmartIdentifier("test-room", roomDef),
		objects: NewInventory(),
	}
	ri.objects.AddObj(oi)

	tests := map[string]struct {
		keyword  string
		wantNil  bool
		wantDesc string
	}{
		"room keyword first": {
			keyword:  "a",
			wantDesc: "desc-ab",
		},
		"room keyword alternate": {
			keyword:  "b",
			wantDesc: "desc-ab",
		},
		"room second extra desc": {
			keyword:  "c",
			wantDesc: "desc-c",
		},
		"object keyword": {
			keyword:  "d",
			wantDesc: "desc-d",
		},
		"case insensitive": {
			keyword:  "A",
			wantDesc: "desc-ab",
		},
		"no match": {
			keyword: "z",
			wantNil: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ed := ri.FindExtraDesc(tc.keyword)
			if tc.wantNil {
				if ed != nil {
					t.Errorf("expected nil, got %q", ed.Description)
				}
				return
			}
			if ed == nil {
				t.Fatal("expected extra desc, got nil")
			}
			if ed.Description != tc.wantDesc {
				t.Errorf("description = %q, want %q", ed.Description, tc.wantDesc)
			}
		})
	}
}

func TestRoomInstance_Describe(t *testing.T) {
	tests := map[string]struct {
		roomName   string
		addObj     bool
		addMob     bool
		addPlayer  bool
		addExit    bool
		actorName  string
		wantInOut  []string
		wantNotOut []string
	}{
		"room name appears in output": {
			roomName:  "The Great Hall",
			wantInOut: []string{"The Great Hall"},
		},
		"object long desc appears": {
			roomName:  "Hall",
			addObj:    true,
			wantInOut: []string{"A shiny sword lies here."},
		},
		"mob appears in output": {
			roomName:  "Hall",
			addMob:    true,
			wantInOut: []string{"goblin"},
		},
		"actor excluded from player list": {
			roomName:   "Hall",
			addPlayer:  true,
			actorName:  "Watcher",
			wantNotOut: []string{"Watcher is here"},
		},
		"other player appears": {
			roomName:  "Hall",
			addPlayer: true,
			actorName: "someone-else",
			wantInOut: []string{"Watcher is here"},
		},
		"room with exits shows exit list": {
			roomName:  "Crossroads",
			addExit:   true,
			wantInOut: []string{"north"},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			dest := newTestRoom("dest")
			var exits map[string]assets.Exit
			if tc.addExit {
				exits = map[string]assets.Exit{
					"north": {Room: storage.NewResolvedSmartIdentifier("dest", &assets.Room{Name: "Dest"})},
				}
			}
			ri, _ := NewRoomInstance(storage.NewResolvedSmartIdentifier("r", &assets.Room{Name: tc.roomName, Exits: exits}))
			if tc.addExit {
				ri.exits["north"].Dest = dest
			}

			if tc.addObj {
				obj := storage.NewResolvedSmartIdentifier("sword", &assets.Object{
					ShortDesc: "a shiny sword",
					LongDesc:  "A shiny sword lies here.",
				})
				oi, _ := NewObjectInstance(obj)
				ri.AddObj(oi)
			}
			if tc.addMob {
				ri.AddMob(newTestMI("g1", "goblin"))
			}
			if tc.addPlayer {
				ri.AddPlayer("watcher", newTestCI("watcher", "Watcher"))
			}

			out := ri.Describe(tc.actorName)

			for _, want := range tc.wantInOut {
				if !strings.Contains(out, want) {
					t.Errorf("Describe() missing %q in:\n%s", want, out)
				}
			}
			for _, notWant := range tc.wantNotOut {
				if strings.Contains(out, notWant) {
					t.Errorf("Describe() should not contain %q but does in:\n%s", notWant, out)
				}
			}
		})
	}
}

// addPlayerToRoom creates a CharacterInstance backed by a buffered msgs channel
// and adds it to room. Returns the instance and the channel.
func addPlayerToRoom(t *testing.T, room *RoomInstance, charId string) (*CharacterInstance, chan []byte) {
	t.Helper()
	msgs := make(chan []byte, 4)
	charRef := storage.NewResolvedSmartIdentifier(charId, &assets.Character{Name: charId})
	ci, err := NewCharacterInstance(charRef, msgs, room)
	if err != nil {
		t.Fatalf("NewCharacterInstance: %v", err)
	}
	room.AddPlayer(charId, ci)
	return ci, msgs
}

func TestRoomInstance_Publish(t *testing.T) {
	tests := map[string]struct {
		exclude []string
		wantInA bool
		wantInB bool
	}{
		"no exclude delivers to all":    {wantInA: true, wantInB: true},
		"exclude one player skips them": {exclude: []string{"a"}, wantInB: true},
		"exclude all skips everyone":    {exclude: []string{"a", "b"}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			room := newTestRoom("r")
			_, msgsA := addPlayerToRoom(t, room, "a")
			_, msgsB := addPlayerToRoom(t, room, "b")

			room.Publish([]byte("hello"), tc.exclude)

			gotA := drainOne(msgsA)
			gotB := drainOne(msgsB)
			if (gotA != "") != tc.wantInA {
				t.Errorf("a got %q, wantDelivered=%v", gotA, tc.wantInA)
			}
			if (gotB != "") != tc.wantInB {
				t.Errorf("b got %q, wantDelivered=%v", gotB, tc.wantInB)
			}
		})
	}
}

func drainOne(ch chan []byte) string {
	select {
	case b := <-ch:
		return string(b)
	default:
		return ""
	}
}
