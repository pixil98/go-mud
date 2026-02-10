package game

import "errors"

var (
	ErrPlayerNotFound = errors.New("player not found")
	ErrPlayerExists   = errors.New("player already exists")
)
