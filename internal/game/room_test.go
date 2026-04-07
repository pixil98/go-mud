package game

import (
	"testing"

	"github.com/pixil98/go-mud/internal/assets"
	"github.com/pixil98/go-mud/internal/storage"
)

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
