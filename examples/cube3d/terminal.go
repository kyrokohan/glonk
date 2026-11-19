//go:build !js

package main

import (
	"context"

	"glonk.dev/engine"
)

// run plays the game in the terminal.
func run(g *cubeGame) error {
	return engine.Run(context.Background(), g, engine.Config{FPS: 30})
}
