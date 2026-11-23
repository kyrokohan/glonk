// Package docs holds the code the guides in this folder show. Each guide's Go is lines of a file
// here or of an example, so go vet builds it and go test runs it; internal/sitepages holds the
// two to each other.
package docs

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"glonk.dev/engine"
)

// A frame is a grid of cells, and each drawing call writes some of them.
func Example_cells() {
	fb, _ := engine.NewFrameBuffer(24, 5)

	fb.DrawBox(0, 0, fb.Width(), fb.Height(), engine.Color{})
	fb.DrawText(2, 1, "glonk", engine.Palette(220))
	fb.Set(2, 3, engine.Cell{Character: '@', Foreground: engine.RGB(255, 135, 0), Attrs: engine.Bold})
	fb.Set(4, 3, engine.Cell{Character: '~', Background: engine.Palette(27)})

	fmt.Println(fb.String())
	// Output:
	// ┌──────────────────────┐
	// │ glonk                │
	// │                      │
	// │ @ ~                  │
	// └──────────────────────┘
}

// sized is a game that is told the size of its frame and how each frame went.
type sized struct {
	width, height int
	stats         engine.Stats
	frames        int
}

func (g *sized) Resize(width, height int) { g.width, g.height = width, height }

func (g *sized) Stats(s engine.Stats) { g.stats = s }

func (g *sized) Update(dt float64, in *engine.Input) error {
	if g.frames++; g.frames == 3 {
		return engine.ErrQuit
	}
	return nil
}

func (g *sized) Draw(fb *engine.FrameBuffer) {
	fb.DrawText(0, 0, fmt.Sprintf("%.0f fps  %d bytes", g.stats.FPS, g.stats.Bytes), engine.Color{})
}

// A game with a Resize method knows the size of its frame, and one with a Stats method how each
// frame went. Here Run has no terminal, so the frame is the size the Config says.
func Example_sizeAndFrameNumbers() {
	g := &sized{}
	var out bytes.Buffer
	err := engine.Run(context.Background(), g, engine.Config{
		Width: 20, Height: 2, FPS: 100,
		Input: strings.NewReader(""), Output: &out,
	})

	fmt.Println(err, g.width, g.height, g.stats.Bytes > 0)
	// Output:
	// <nil> 20 2 true
}
