// Command cube3d is a walk around a floor with six shaded cubes and a wireframe one.
//
//	go run ./examples/cube3d
//
// W and S move forward and back, A and D strafe, Q, E and the left and right
// arrows turn, and the up and down arrows look up and down. M switches between
// character shading and half-block pixels (the wireframe cube becomes pixel lines too,
// and the floor stays dotted), B switches between exact and solid pixel edges, and Esc
// quits. Letters work in either case. Where the terminal reports no key releases only one
// key stays held, so walking and turning together does not work, and the game says so.
//
// In half-block mode a tree stands on the floor: a picture drawn with
// bitmap.Bitmap.DrawScaledDepth at the pixels and the depth that mesh.Camera.ProjectPixels
// gives its foot and top, so the cubes hide it where they are nearer and it hides what is
// behind it.
package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"time"
	"unicode"

	"glonk.dev/bitmap"
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

// The tree: a picture of 9 by 12 pixels drawn here from text, so that the program needs no
// file. . is see-through, g and d are leaves and t is the trunk, which is three pixels wide
// because a picture stretched down with no smoothing loses a feature that is one pixel wide.
// It stands on the floor at treeAt, treeHeight tall.
var treeArt = []string{
	"...ggg...",
	"..ggggg..",
	".gggdggg.",
	".ggggggg.",
	"ggdgggggg",
	"ggggggdgg",
	".ggggggg.",
	"..ggdgg..",
	"...ttt...",
	"...ttt...",
	"...ttt...",
	"..ttttt..",
}

var (
	treeLeaf  = engine.RGB(50, 180, 70)
	treeDark  = engine.RGB(25, 120, 45)
	treeTrunk = engine.RGB(120, 80, 40)
)

var treeAt = geometry.Vector3{X: 3, Y: floorY, Z: 3}

const treeHeight = 3.0

// newTree returns the picture of the tree, made from treeArt. A character that is not a
// color of the tree, the dots, is left see-through.
func newTree() *bitmap.Bitmap {
	tree := bitmap.New(len(treeArt[0]), len(treeArt))
	for y, row := range treeArt {
		for x, r := range row {
			switch r {
			case 'g':
				tree.Set(x, y, treeLeaf)
			case 'd':
				tree.Set(x, y, treeDark)
			case 't':
				tree.Set(x, y, treeTrunk)
			}
		}
	}
	return tree
}

type cubeGame struct {
	cam   mesh.Camera
	floor mesh.Mesh
	cubes []mesh.Mesh
	ghost mesh.Mesh    // a cube drawn as a wireframe (pixel lines in half-block mode), partly hidden by the solid ones in front of it
	stats engine.Stats // how the last frame went, given by Run after every frame

	// halfBlocks draws the cubes as pixels, two per cell, shaded by color. Off, they
	// are drawn one per cell, shaded by character. M toggles it.
	halfBlocks bool

	// flipEdges is set when B is pressed. The next half-block frame then switches the
	// pixel edges between exact and solid. The library picks the default for the
	// terminal and keeps the setting from frame to frame and across resizes. The
	// viewer has the same few lines.
	flipEdges bool

	// tree is the picture that stands in the scene in half-block mode.
	tree *bitmap.Bitmap

	// noReleases is set once Update has seen that the terminal reports no key releases, where
	// only one key stays held, so that walking and turning together does not work. The HUD
	// then says so. It is false until the first Update has asked.
	noReleases bool
}

// keyDown reports whether a letter key counts as down or pressed, in either case.
// Terminals that cannot report keys exactly send a capital for Shift or Caps Lock.
// held is Input.IsDown or Input.Started.
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
	g.noReleases = !in.Exact()
	if in.Pressed(engine.KeyEscape) {
		return engine.ErrQuit
	}
	// Started, not Pressed: these switch something, and a held key must switch it once.
	if keyDown(in.Started, 'm') {
		g.halfBlocks = !g.halfBlocks
	}
	if g.halfBlocks && keyDown(in.Started, 'b') {
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

	return nil
}

var _ engine.StatsReceiver = (*cubeGame)(nil)

// Stats keeps how the last frame went, for the lines at the top and the bottom.
func (g *cubeGame) Stats(s engine.Stats) { g.stats = s }

// ms is a duration in milliseconds.
func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

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
		g.drawTree(canvas)
		// The ghost is a wireframe with depth, drawn into the same pixels as the solids: a
		// cube in front of it hides the part of its lines that it covers, to the pixel.
		g.ghost.DrawDepthPixels(canvas, g.cam, g.ghost.Material.Color)
		// Compose before the HUD so the HUD lands on top of the pixels.
		canvas.Compose()
	} else {
		for _, o := range g.cubes {
			o.DrawShaded(fb, g.cam, lights...)
		}
		// The ghost is a wireframe drawn with depth, after the solids: a cube in front of it
		// hides the part of its edges that it covers.
		g.ghost.DrawDepth(fb, g.cam, engine.Cell{Character: '#', Foreground: g.ghost.Material.Color})
	}

	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})
	fb.DrawText(1, 1, fmt.Sprintf("%.0f fps  %s", g.stats.FPS, mode), engine.Color{})
	if g.noReleases {
		fb.DrawText(1, 2, "no key releases here: one key at a time", engine.Color{})
	}
	fb.DrawText(1, fb.Height()-4, fmt.Sprintf("update %.1f  draw %.1f  write %.1f ms  %.1f KB sent", ms(g.stats.Update), ms(g.stats.Draw), ms(g.stats.Write), float64(g.stats.Bytes)/1000), engine.Color{})
	fb.DrawText(1, fb.Height()-3, "WASD move  Q/E turn  arrows look", engine.Color{})
	fb.DrawText(1, fb.Height()-2, "M mode  B edges  Esc quit", engine.Color{})
}

// drawTree stands the tree on the floor at treeAt, treeHeight tall, as the camera sees it.
// The foot and the top are projected to pixels and the picture is stretched between them,
// keeping its shape. Its depth is one number for the whole picture, the mean of the foot's and
// the top's, which is what DrawScaledDepth takes. A tree the camera cannot see is not drawn.
func (g *cubeGame) drawTree(c *engine.Canvas) {
	if g.tree == nil {
		return
	}
	footX, footY, footDepth, ok := g.cam.ProjectPixels(treeAt, c)
	if !ok {
		return
	}
	_, topY, topDepth, ok := g.cam.ProjectPixels(treeAt.Add(geometry.Vector3{Y: treeHeight}), c)
	if !ok {
		return
	}
	h := footY - topY
	w := h * float64(g.tree.Width()) / float64(g.tree.Height())
	// Floor the corner, as ProjectPixels says, and not a conversion to int.
	g.tree.DrawScaledDepth(c, int(math.Floor(footX-w/2)), int(math.Floor(topY)), int(math.Round(w)), int(math.Round(h)), (footDepth+topDepth)/2)
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
		tree:  newTree(),
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
