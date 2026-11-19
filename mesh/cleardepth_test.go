package mesh

import (
	"testing"

	"glonk.dev/engine"
)

// checkDrawnOver checks a frame drawn as a scene, ClearDepth, and then two cubes, against the cubes
// drawn alone on an empty frame and the scene drawn alone: where the cubes put something, over is
// what they put, and everywhere else it is the scene. farOnly is the far cube alone, to show that
// the near one hides part of it and not all of it, so that their own depth test is what is checked.
func checkDrawnOver[T comparable](t *testing.T, over, alone, scene, farOnly []T, empty T) {
	t.Helper()
	hidden, shown := 0, 0
	for i := range over {
		want := alone[i]
		if want == empty {
			want = scene[i]
		}
		if over[i] != want {
			t.Fatalf("at %d: wanted %v, got %v", i, want, over[i])
		}
		if farOnly[i] != empty {
			if alone[i] == farOnly[i] {
				shown++
			} else {
				hidden++
			}
		}
	}
	if hidden == 0 || shown == 0 {
		t.Fatalf("wanted the near cube to hide part of the far one, got %d hidden and %d shown", hidden, shown)
	}
}

// A mesh drawn after ClearDepth lands on top of a nearer scene as it would on an empty frame, its
// own faces hiding each other, and the scene is kept where the mesh is not.
func TestClearDepthDrawsOverANearerScene(t *testing.T) {
	cam := Camera{}
	light := Ambient{Level: 1}
	wall := farWall(3)
	near := solidCube(vec(0, 0, 6), 2)
	far := solidCube(vec(0.8, 0.5, 8), 2).withColor(engine.Palette(45))

	t.Run("pixels", func(t *testing.T) {
		draw := func(scene bool, meshes ...Mesh) []engine.Color {
			c := newShadeBuffer(t).Canvas()
			if scene {
				wall.DrawShadedPixels(c, cam, light)
				c.ClearDepth()
			}
			for _, m := range meshes {
				m.DrawShadedPixels(c, cam, light)
			}
			out := make([]engine.Color, 0, c.Width()*c.Height())
			for y := range c.Height() {
				for x := range c.Width() {
					out = append(out, c.At(x, y))
				}
			}
			return out
		}
		// The far cube is drawn after the near one, so only depth keeps it behind.
		checkDrawnOver(t, draw(true, near, far), draw(false, near, far), draw(true), draw(false, far), engine.Color{})
	})

	t.Run("cells", func(t *testing.T) {
		draw := func(scene bool, meshes ...Mesh) []engine.Cell {
			fb := newShadeBuffer(t)
			if scene {
				wall.DrawShaded(fb, cam, light)
				fb.ClearDepth()
			}
			for _, m := range meshes {
				m.DrawShaded(fb, cam, light)
			}
			out := make([]engine.Cell, 0, fb.Width()*fb.Height())
			for y := range fb.Height() {
				for x := range fb.Width() {
					out = append(out, fb.At(x, y))
				}
			}
			return out
		}
		checkDrawnOver(t, draw(true, near, far), draw(false, near, far), draw(true), draw(false, far), engine.BlankCell)
	})
}
