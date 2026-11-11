package widget

import (
	"testing"

	"glonk.dev/engine"
)

// Drawing happens every frame, so it must not allocate.
func TestDrawingAllocatesNothing(t *testing.T) {
	fb := frame(t, 40, 10)
	label := Label{Text: "coins 3/12", Align: Center, Style: engine.Cell{Foreground: engine.Palette(3), Background: engine.Palette(4), Attrs: engine.Bold}}
	panel := Panel{Title: "Status", Style: engine.Cell{Foreground: engine.Palette(2)}, Fill: engine.Cell{Background: engine.Palette(1)}}
	bar := Bar{Value: 0.4}

	draws := map[string]func(){
		"label": func() { label.Draw(fb, 2, 1, 30) },
		"panel": func() { panel.Draw(fb, 0, 0, 40, 10) },
		"bar":   func() { bar.Draw(fb, 2, 2, 30) },
	}
	for name, draw := range draws {
		if n := testing.AllocsPerRun(50, draw); n != 0 {
			t.Errorf("wanted %s.Draw to allocate nothing, got %v allocations", name, n)
		}
	}
}
