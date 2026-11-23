package docs

import (
	"fmt"
	"strings"

	"glonk.dev/bitmap"
	"glonk.dev/engine"
	"glonk.dev/geometry"
	"glonk.dev/mesh"
	"glonk.dev/sprite"
	"glonk.dev/tilemap"
)

// The code of the six steps on the cover of glonk.dev, each run here. internal/sitepages holds
// the page to these lines.

func Example_landingABlock() {
	fb, _ := engine.NewFrameBuffer(24, 6)

	fb.Set(10, 5, engine.Cell{Character: '@', Foreground: engine.Palette(208)})

	canvas := fb.Canvas() // two pixels to a cell
	canvas.FillRect(2, 1, 20, 6, engine.Palette(220))
	canvas.Compose()

	show(canvas)
	fmt.Printf("%c\n", fb.At(10, 5).Character)
	// Output:
	// ........................
	// ..####################..
	// ..####################..
	// ..####################..
	// ..####################..
	// ..####################..
	// ..####################..
	// ........................
	// ........................
	// ........................
	// ........................
	// ........................
	// @
}

func Example_landingSprites() {
	fb, _ := engine.NewFrameBuffer(8, 4)

	ship := sprite.FromLines(engine.Cell{Foreground: engine.Palette(208)},
		` /\ `,
		`/__\`,
	)
	layer := sprite.NewLayer()
	layer.Add(&sprite.Sprite{
		Position: geometry.Vector2{X: 2, Y: 1},
		Frame:    ship,
		Overlay:  true, // see-through where the art has a space
	})
	layer.Draw(fb)

	fmt.Println(fb)
	// Output:
	// /\
	//   /__\
}

func Example_landingALevel() {
	fb, _ := engine.NewFrameBuffer(6, 3)
	player := sprite.New(geometry.Vector2{X: 1, Y: 1}, '@')
	dx, dy := 3.0, 0.0

	level := tilemap.FromLines(map[rune]tilemap.Tile{
		'#': {Cell: engine.Cell{Character: '#'}, Solid: true},
		'.': {Cell: engine.Cell{Character: '.'}},
	}, "##########", "#........#", "##########")

	moved, _, _ := level.Move(player.Bounds(), geometry.Vector2{X: dx, Y: dy})
	player.Position = moved.Min

	screen := geometry.NewRect(0, 0, float64(fb.Width()), float64(fb.Height()))
	view := screen.CenterOn(player.Bounds().Center()).Within(level.Bounds())

	fmt.Println(player.Position.X, view.Min.X)
	// Output:
	// 4 1.5
}

func Example_landingSolidsAndLight() {
	fb, _ := engine.NewFrameBuffer(30, 9)

	cube := mesh.Cube(2)
	cube.Material.Color = engine.Palette(208)
	cam := mesh.Camera{Position: geometry.Vector3{Z: -4}}
	sun := mesh.Directional{Direction: geometry.Vector3{X: 0.4, Y: -1, Z: 0.6}}

	cube.DrawShaded(fb, cam, mesh.Ambient{Level: 0.2}, sun)

	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})
	fmt.Println(fb)
	// Output:
	// ┌────────────────────────────┐
	// │                            │
	// │                            │
	// │           ******           │
	// │           ******           │
	// │           ******           │
	// │                            │
	// │                            │
	// └────────────────────────────┘
}

func Example_landingBothInOneFrame() {
	fb, _ := engine.NewFrameBuffer(30, 9)
	cube := mesh.Cube(2)
	cam := mesh.Camera{Position: geometry.Vector3{X: -1, Z: -4}}
	lights := []mesh.Light{mesh.Ambient{Level: 1}}
	tree := bitmap.New(2, 4)
	tree.Set(0, 0, engine.RGB(0, 160, 0))
	x, y, w, h, depth := 2, 2, 2, 4, 5.0

	canvas := fb.Canvas()
	cube.DrawShadedPixels(canvas, cam, lights...)   // solids, in pixels
	tree.DrawScaledDepth(canvas, x, y, w, h, depth) // a picture among them
	canvas.Compose()
	fb.DrawText(1, 1, "25 fps", engine.Color{}) // and text on top

	fmt.Println(strings.Split(fb.String(), "\n")[1])
	fmt.Println("the tree:", canvas.At(2, 2))
	// Output:
	// 25 fps
	// the tree: #00a000
}

// walker walks right while W is held.
type walker struct{ x int }

func (g *walker) Update(dt float64, in *engine.Input) error {
	if in.IsDown('w') {
		g.x++
	}
	return nil
}

func (g *walker) Draw(fb *engine.FrameBuffer) {
	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})
	fb.Set(1+g.x, 1, engine.Cell{Character: '@'})
}

func Example_landingKeysAndTests() {
	g := &walker{}

	kb := engine.NewKeyboard()
	kb.Press('w')
	g.Update(1.0/30, kb.Input()) // one frame with W held

	fb, _ := engine.NewFrameBuffer(40, 12)
	g.Draw(fb)
	fmt.Print(fb.String()) // the frame, as text
	// Output:
	// ┌──────────────────────────────────────┐
	// │ @                                    │
	// │                                      │
	// │                                      │
	// │                                      │
	// │                                      │
	// │                                      │
	// │                                      │
	// │                                      │
	// │                                      │
	// │                                      │
	// └──────────────────────────────────────┘
}
