package game

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

func testMob(id string) *MobileInstance {
	mi, _ := NewMobileInstance(storage.NewResolvedSmartIdentifier(id, &assets.Mobile{ShortDesc: id}))
	return mi
}

func TestThreatTable_EnsureEntry(t *testing.T) {
	tests := map[string]struct {
		preloadThreat int
		wantVal       int
	}{
		"new entry gets threat 1": {
			wantVal: 1,
		},
		"existing entry preserved": {
			preloadThreat: 50,
			wantVal:       50,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var tt ThreatTable
			mob := testMob("a")
			if tc.preloadThreat > 0 {
				tt.ensureEntry("a", mob)
				tt.entries["a"].threat = tc.preloadThreat
			}
			tt.ensureEntry("a", mob)
			if got := tt.entries["a"].threat; got != tc.wantVal {
				t.Errorf("threat = %d, want %d", got, tc.wantVal)
			}
		})
	}
}

func TestThreatTable_AddThreat(t *testing.T) {
	var tt ThreatTable
	tt.ensureEntry("a", testMob("a"))
	tt.addThreat("a", 10)
	if got := tt.entries["a"].threat; got != 11 {
		t.Errorf("threat = %d, want 11", got)
	}
	tt.addThreat("a", 5)
	if got := tt.entries["a"].threat; got != 16 {
		t.Errorf("threat = %d, want 16", got)
	}
}

func TestThreatTable_SetThreat(t *testing.T) {
	var tt ThreatTable
	tt.ensureEntry("a", testMob("a"))
	tt.setThreat("a", 42)
	if got := tt.entries["a"].threat; got != 42 {
		t.Errorf("threat = %d, want 42", got)
	}
}

func TestThreatTable_TopThreat(t *testing.T) {
	var tt ThreatTable
	tt.ensureEntry("a", testMob("a"))
	tt.ensureEntry("b", testMob("b"))
	tt.entries["b"].threat = 20
	tt.topThreat("a")
	if got := tt.entries["a"].threat; got != 21 {
		t.Errorf("threat = %d, want 21", got)
	}
}

func TestThreatTable_HasEntry(t *testing.T) {
	var tt ThreatTable
	tt.ensureEntry("a", testMob("a"))
	if !tt.hasEntry("a") {
		t.Error("expected hasEntry(a) = true")
	}
	if tt.hasEntry("b") {
		t.Error("expected hasEntry(b) = false")
	}
}

func TestThreatTable_HasEntries(t *testing.T) {
	empty := ThreatTable{}
	if empty.hasEntries() {
		t.Error("expected empty table to have no entries")
	}
	var full ThreatTable
	full.ensureEntry("a", testMob("a"))
	if !full.hasEntries() {
		t.Error("expected non-empty table to have entries")
	}
}

func TestThreatTable_RemoveEntry(t *testing.T) {
	var tt ThreatTable
	tt.ensureEntry("a", testMob("a"))
	tt.ensureEntry("b", testMob("b"))
	tt.removeEntry("a")
	if tt.hasEntry("a") {
		t.Error("expected a to be removed")
	}
	if !tt.hasEntry("b") {
		t.Error("expected b to remain")
	}
}

func TestThreatTable_Snapshot(t *testing.T) {
	tests := map[string]struct {
		entries map[string]int
		wantNil bool
	}{
		"empty table": {
			wantNil: true,
		},
		"populated table": {
			entries: map[string]int{"a": 10, "b": 20},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var tt ThreatTable
			for id, threat := range tc.entries {
				tt.ensureEntry(id, testMob(id))
				tt.entries[id].threat = threat
			}
			snap := tt.snapshot()
			if tc.wantNil {
				if snap != nil {
					t.Errorf("expected nil, got %v", snap)
				}
				return
			}
			for k, v := range tc.entries {
				if snap[k] != v {
					t.Errorf("snapshot[%q] = %d, want %d", k, snap[k], v)
				}
			}
		})
	}
}

func TestThreatTable_Clear(t *testing.T) {
	var tt ThreatTable
	tt.ensureEntry("a", testMob("a"))
	tt.clear()
	if tt.hasEntries() {
		t.Error("expected empty after clear")
	}
}

func TestThreatTable_ResolveTarget(t *testing.T) {
	t.Run("empty table", func(t *testing.T) {
		var tt ThreatTable
		if got := tt.resolveTarget(""); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("highest threat wins", func(t *testing.T) {
		var tt ThreatTable
		a := testMob("a")
		b := testMob("b")
		tt.ensureEntry(a.Id(), a)
		tt.ensureEntry(b.Id(), b)
		tt.entries[b.Id()].threat = 20
		got := tt.resolveTarget("")
		if got == nil || got.Id() != b.Id() {
			t.Errorf("expected b, got %v", got)
		}
	})

	t.Run("preferred overrides highest", func(t *testing.T) {
		var tt ThreatTable
		a := testMob("a")
		b := testMob("b")
		tt.ensureEntry(a.Id(), a)
		tt.ensureEntry(b.Id(), b)
		tt.entries[b.Id()].threat = 20
		got := tt.resolveTarget(a.Id())
		if got == nil || got.Id() != a.Id() {
			t.Errorf("expected a, got %v", got)
		}
	})

	t.Run("preferred not in table falls back", func(t *testing.T) {
		var tt ThreatTable
		a := testMob("a")
		b := testMob("b")
		tt.ensureEntry(a.Id(), a)
		tt.ensureEntry(b.Id(), b)
		tt.entries[b.Id()].threat = 20
		got := tt.resolveTarget("gone")
		if got == nil || got.Id() != b.Id() {
			t.Errorf("expected b, got %v", got)
		}
	})
}

func TestThreatTable_Enemies(t *testing.T) {
	var tt ThreatTable
	tt.ensureEntry("a", testMob("a"))
	tt.ensureEntry("b", testMob("b"))
	enemies := tt.enemies()
	if len(enemies) != 2 {
		t.Errorf("enemies count = %d, want 2", len(enemies))
	}
}
