package game

import "testing"

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
			mob := newTestMI("a", "A")
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
	tt.ensureEntry("a", newTestMI("a", "A"))
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
	tt.ensureEntry("a", newTestMI("a", "A"))
	tt.setThreat("a", 42)
	if got := tt.entries["a"].threat; got != 42 {
		t.Errorf("threat = %d, want 42", got)
	}
}

func TestThreatTable_TopThreat(t *testing.T) {
	var tt ThreatTable
	tt.ensureEntry("a", newTestMI("a", "A"))
	tt.ensureEntry("b", newTestMI("b", "B"))
	tt.entries["b"].threat = 20
	tt.topThreat("a")
	if got := tt.entries["a"].threat; got != 21 {
		t.Errorf("threat = %d, want 21", got)
	}
}

func TestThreatTable_HasEntry(t *testing.T) {
	var tt ThreatTable
	tt.ensureEntry("a", newTestMI("a", "A"))
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
	full.ensureEntry("a", newTestMI("a", "A"))
	if !full.hasEntries() {
		t.Error("expected non-empty table to have entries")
	}
}

func TestThreatTable_RemoveEntry(t *testing.T) {
	var tt ThreatTable
	tt.ensureEntry("a", newTestMI("a", "A"))
	tt.ensureEntry("b", newTestMI("b", "B"))
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
				tt.ensureEntry(id, newTestMI(id, id))
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
	tt.ensureEntry("a", newTestMI("a", "A"))
	tt.clear()
	if tt.hasEntries() {
		t.Error("expected empty after clear")
	}
}

func TestThreatTable_Enemies(t *testing.T) {
	var tt ThreatTable
	tt.ensureEntry("a", newTestMI("a", "A"))
	tt.ensureEntry("b", newTestMI("b", "B"))
	enemies := tt.enemies()
	if len(enemies) != 2 {
		t.Errorf("enemies count = %d, want 2", len(enemies))
	}
}

func TestThreatTable_ResolveTarget(t *testing.T) {
	tests := map[string]struct {
		setup     func() ThreatTable
		preferred string
		wantId    string // empty means expect nil
	}{
		"empty table returns nil": {
			setup:  func() ThreatTable { return ThreatTable{} },
			wantId: "",
		},
		"highest threat wins": {
			setup: func() ThreatTable {
				var tt ThreatTable
				tt.ensureEntry("a", newTestMI("a", "A"))
				tt.ensureEntry("b", newTestMI("b", "B"))
				tt.entries["b"].threat = 20
				return tt
			},
			wantId: "b",
		},
		"preferred target overrides highest threat": {
			setup: func() ThreatTable {
				var tt ThreatTable
				tt.ensureEntry("a", newTestMI("a", "A"))
				tt.ensureEntry("b", newTestMI("b", "B"))
				tt.entries["b"].threat = 20
				return tt
			},
			preferred: "a",
			wantId:    "a",
		},
		"preferred not in table falls back to highest": {
			setup: func() ThreatTable {
				var tt ThreatTable
				tt.ensureEntry("a", newTestMI("a", "A"))
				tt.ensureEntry("b", newTestMI("b", "B"))
				tt.entries["b"].threat = 20
				return tt
			},
			preferred: "gone",
			wantId:    "b",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tt := tc.setup()
			got := tt.resolveTarget(tc.preferred)
			if tc.wantId == "" {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected %q, got nil", tc.wantId)
			}
			if got.Id() != tc.wantId {
				t.Errorf("Id = %q, want %q", got.Id(), tc.wantId)
			}
		})
	}
}


