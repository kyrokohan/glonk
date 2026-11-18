package main

import (
	"testing"

	"glonk.dev/engine"
	"glonk.dev/internal/record"
)

// TestRecord writes this example's recording when the -record flag asks for it: see
// internal/record.
func TestRecord(t *testing.T) {
	record.Save(t, "sprites", newSpriteGame(), record.Tape{
		Width: 60, Height: 20, FPS: 25, Frames: 150,
		Steps: record.Steps(
			record.Hold(engine.KeyRight, 5, 40),
			record.Hold(engine.KeyDown, 30, 60),
			record.Hold(engine.KeyLeft, 60, 100),
			record.Hold(engine.KeyUp, 90, 130),
		),
	})
}
