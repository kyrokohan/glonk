// Command cube3d is a walk around a floor with six shaded cubes and a wireframe one.
//
//	go run ./examples/cube3d
//
// W and S move forward and back, A and D strafe, Q, E and the left and right
// arrows turn, and the up and down arrows look up and down. M switches between
// character shading and half-block pixels, B switches between exact and solid pixel
// edges, and Esc quits. Letters work in either case.
package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"unicode"

	"glonk.dev/engine"
	"glonk.dev/geometry"
	"glonk.dev/mesh"
)

const (
	gridSize      = 40
	gridDivisions = 20 // 2x2 floor cells, so a size-2 cube fills one
	floorY        = -1.5
	walkLimit     = gridSize/2 - 0.5 // invisible walls, half a unit inside the floor edge
)

// lights light the cubes: some ambient fill, and a sun from above, to one side and
// slightly ahead of the start.
var lights = []mesh.Light{
	mesh.Ambient{Level: 0.2},
	mesh.Directional{Direction: geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}},
}

type cubeGame struct {
	cam   mesh.Camera
	floor mesh.Mesh
	cubes []mesh.Mesh
	ghost mesh.Mesh // a cube drawn as a wireframe, partly hidden by the solid ones in front of it
	fps   float64

	// halfBlocks draws the cubes as pixels, two per cell, shaded by color. Off, they
	// are drawn one per cell, shaded by character. M toggles it.
	halfBlocks bool

	// flipEdges is set when B is pressed. The next half-block frame then switches the
	// pixel edges between exact and solid. The library picks the default for the
	// terminal and keeps the setting from frame to frame and across resizes. The
	// viewer has the same few lines.
	flipEdges bool
}

// keyDown reports whether a letter key counts as down or pressed, in either case.
// Terminals that cannot report keys exactly send a capital for Shift or Caps Lock.
// held is Input.IsDown or Input.Pressed.
func keyDown(held func(engine.Key) bool, letter rune) bool {
	return held(engine.Key(letter)) || held(engine.Key(unicode.ToUpper(letter)))
}

// newCube returns a cube of the given size resting on the floor at (x, z).
func newCube(size, x, z float64, color engine.Color) mesh.Mesh {
	m := mesh.Cube(size)
	m.Transform.Position = geometry.Vector3{X: x, Y: floorY + size/2, Z: z}
	m.Material.Color = color
	return m
}

func (g *cubeGame) Update(dt float64, in *engine.Input) error {
	if in.Pressed(engine.KeyEscape) {
		return engine.ErrQuit
	}
	if keyDown(in.Pressed, 'm') {
		g.halfBlocks = !g.halfBlocks
	}
	if g.halfBlocks && keyDown(in.Pressed, 'b') {
		g.flipEdges = true
	}

	const moveSpeed = 3.0
	const turnSpeed = 1.5
	const pitchSpeed = 1.0
	const maxPitch = 1.5 // just short of straight up or down, where the view would flip

	move := geometry.Vector3{}
	if keyDown(in.IsDown, 'w') {
		move = move.Add(g.cam.Forward())
	}
	if keyDown(in.IsDown, 's') {
		move = move.Sub(g.cam.Forward())
	}
	if keyDown(in.IsDown, 'd') {
		move = move.Add(g.cam.Right())
	}
	if keyDown(in.IsDown, 'a') {
		move = move.Sub(g.cam.Right())
	}
	// Normalized so moving diagonally is not ~1.41x faster.
	g.cam.Position = g.cam.Position.Add(move.Normalize().Scale(moveSpeed * dt))
	g.cam.Position.X = max(-walkLimit, min(walkLimit, g.cam.Position.X))
	g.cam.Position.Z = max(-walkLimit, min(walkLimit, g.cam.Position.Z))

	if keyDown(in.IsDown, 'e') || in.IsDown(engine.KeyRight) {
		g.cam.Yaw += turnSpeed * dt
	}
	if keyDown(in.IsDown, 'q') || in.IsDown(engine.KeyLeft) {
		g.cam.Yaw -= turnSpeed * dt
	}
	if in.IsDown(engine.KeyUp) {
		g.cam.Pitch += pitchSpeed * dt
	}
	if in.IsDown(engine.KeyDown) {
		g.cam.Pitch -= pitchSpeed * dt
	}
	g.cam.Pitch = max(-maxPitch, min(maxPitch, g.cam.Pitch))

	g.fps = 0.9*g.fps + 0.1*(1/dt)
	return nil
}

func (g *cubeGame) Draw(fb *engine.FrameBuffer) {
	// The floor is wireframe and ignores depth, so draw it first; the cubes then
	// cover it where they stand, and hide each other by depth.
	g.floor.Draw(fb, g.cam, engine.Cell{Character: '.', Foreground: engine.Palette(240)})

	mode := "glyphs"
	if g.halfBlocks {
		canvas := fb.Canvas()
		if g.flipEdges {
			canvas.Seamless = !canvas.Seamless
			g.flipEdges = false
		}
		mode = "half-blocks, exact edges"
		if canvas.Seamless {
			mode = "half-blocks, solid edges"
		}
		for _, o := range g.cubes {
			o.DrawShadedPixels(canvas, g.cam, lights...)
		}
		// Compose before the HUD so the HUD lands on top of the pixels.
		canvas.Compose()
	} else {
		for _, o := range g.cubes {
			o.DrawShaded(fb, g.cam, lights...)
		}
	}

	// The ghost is a wireframe drawn with depth, after the solids: a cube in front of it
	// hides the part of its edges that it covers. Pixels from the half-block mode hide it
	// too, because Compose gives the cells they fill their depth.
	g.ghost.DrawDepth(fb, g.cam, engine.Cell{Character: '#', Foreground: g.ghost.Material.Color})

	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})
	fb.DrawText(1, 1, fmt.Sprintf("%.0f fps  %s", g.fps, mode), engine.Color{})
	fb.DrawText(1, fb.Height()-3, "WASD move  Q/E turn  arrows look", engine.Color{})
	fb.DrawText(1, fb.Height()-2, "M mode  B edges  Esc quit", engine.Color{})
}

// newCubeGame returns the scene main runs: a floor, six cubes and a wireframe ghost,
// seen from just inside the south edge.
func newCubeGame() *cubeGame {
	floor := mesh.Grid(gridSize, gridDivisions)
	floor.Transform.Position.Y = floorY

	return &cubeGame{
		cam:   mesh.Camera{Position: geometry.Vector3{Z: -4}, FOV: math.Pi / 2, Near: 0.1},
		floor: floor,
		cubes: []mesh.Mesh{
			newCube(2, 0, 6, engine.Palette(208)),
			newCube(1, -4, 4, engine.Palette(39)),
			newCube(3, 6, 10, engine.Palette(118)),
			newCube(1.5, -8, 12, engine.Palette(201)),
			newCube(2, 10, -4, engine.Palette(39)),
			newCube(1, -6, -8, engine.Palette(118)),
		},
		ghost: newCube(2.6, 1.6, 8.6, engine.Palette(51)), // clear of every solid, partly behind the orange one from the start
		fps:   30,
	}
}

func main() {
	err := engine.Run(context.Background(), newCubeGame(), engine.Config{
		FPS: 30,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
