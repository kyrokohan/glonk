package main

import (
	"testing"

	"glonk.dev/internal/record"
)

// TestRecord writes this example's recording when the -record flag asks for it: see
// internal/record. The tape is the walk shown at the top of the README.
func TestRecord(t *testing.T) {
	record.Save(t, "cube3d", newCubeGame(), record.Tape{
		Width: 120, Height: 40, FPS: 25, Frames: 160,
		Steps: record.Steps(
			record.Tap('m', 0), // half-block pixels, where the tree stands
			record.Hold('w', 8, 48),
			record.Hold('e', 48, 58),
			record.Hold('w', 58, 88), // past the tree
			record.Hold('q', 88, 96),
			record.Hold('w', 96, 122), // into the wireframe cube
		),
	})
}
