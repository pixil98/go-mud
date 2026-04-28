package game

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

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
		exclude    []string
		wantInA    bool
		wantInB    bool
	}{
		"no exclude delivers to all":        {wantInA: true, wantInB: true},
		"exclude one player skips them":     {exclude: []string{"a"}, wantInB: true},
		"exclude all skips everyone":        {exclude: []string{"a", "b"}},
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
