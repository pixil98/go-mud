package game

import "errors"

var (
	ErrPlayerNotFound    = errors.New("player not found")
	ErrPlayerExists      = errors.New("player already exists")
	ErrPlayerReconnected = errors.New("player reconnected from another session")
)
