package main

import (
	"testing"

	"glonk.dev/engine"
	"glonk.dev/internal/record"
)

// TestRecord writes this example's recording when the -record flag asks for it: see
// internal/record.
func TestRecord(t *testing.T) {
	record.Save(t, "world", newWorldGame(), record.Tape{
		Width: 60, Height: 20, FPS: 25, Frames: 130,
		Steps: record.Steps(
			record.Hold(engine.KeyDown, 5, 11),
			record.Hold(engine.KeyRight, 11, 48), // along the hall, and the camera follows
			record.Hold(engine.KeyDown, 48, 72),  // down the passage to the pool
			record.Hold(engine.KeyRight, 72, 84),
			record.Hold(engine.KeyDown, 84, 90),
			record.Tap(engine.KeyEscape, 104), // the pause menu
		),
	})
}
