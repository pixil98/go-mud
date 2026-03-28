package game

import "errors"

var (
	// ErrPlayerNotFound is returned when a requested player cannot be located in the world.
	ErrPlayerNotFound    = errors.New("player not found")
	// ErrPlayerExists is returned when a player with the same ID is already in the world.
	ErrPlayerExists      = errors.New("player already exists")
	// ErrPlayerReconnected is returned when a player reconnects from a different session.
	ErrPlayerReconnected = errors.New("player reconnected from another session")
)
