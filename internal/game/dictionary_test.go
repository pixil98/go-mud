package game

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
)

func TestDictionary_Resolve(t *testing.T) {
	tests := map[string]struct {
		name string
	}{
		"empty dictionary resolves without error": {},
	}
	for name := range tests {
		t.Run(name, func(t *testing.T) {
			d := &Dictionary{
				Mobiles: newFakeStore[*assets.Mobile](nil),
				Objects: newFakeStore[*assets.Object](nil),
				Rooms:   newFakeStore[*assets.Room](nil),
				Zones:   newFakeStore[*assets.Zone](nil),
			}
			if err := d.Resolve(); err != nil {
				t.Errorf("Resolve: %v", err)
			}
		})
	}
}
