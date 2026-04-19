package game

import "testing"

func TestSinglePlayer_ForEachPlayer(t *testing.T) {
	tests := map[string]struct {
		charId string
	}{
		"calls fn with given id": {charId: "player-123"},
		"empty string id":        {charId: ""},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			pg := SinglePlayer(tc.charId)

			var gotId string
			var gotCi *CharacterInstance
			called := 0
			pg.ForEachPlayer(func(id string, ci *CharacterInstance) {
				gotId = id
				gotCi = ci
				called++
			})

			if called != 1 {
				t.Errorf("ForEachPlayer called %d times, want 1", called)
			}
			if gotId != tc.charId {
				t.Errorf("id = %q, want %q", gotId, tc.charId)
			}
			if gotCi != nil {
				t.Errorf("ci = %v, want nil", gotCi)
			}
		})
	}
}
