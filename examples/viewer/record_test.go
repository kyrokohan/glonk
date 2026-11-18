package main

import (
	"testing"

	"glonk.dev/internal/record"
)

// TestRecord writes this example's recording when the -record flag asks for it: see
// internal/record.
func TestRecord(t *testing.T) {
	models, err := browseModels(nil)
	if err != nil {
		t.Fatal(err)
	}
	record.Save(t, "viewer", newViewer(models), record.Tape{
		Width: 120, Height: 40, FPS: 25, Frames: 200,
		Steps: record.Steps(
			record.Tap('m', 0),      // half-block pixels
			record.Hold('w', 2, 14), // closer
			record.Tap('n', 50),     // the icosphere
			record.Tap('f', 72),     // smoothed
			record.Tap('n', 95),     // the octahedron, a color at each corner
			record.Tap('n', 135),    // the torus
			record.Tap('v', 168),    // as a wireframe
		),
	})
}
