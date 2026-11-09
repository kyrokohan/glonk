package sprite

import (
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// floor returns a buffer w cells wide and one tall, every cell a '.' on a green background.
func floor(t *testing.T, w int) *engine.FrameBuffer {
	t.Helper()
	fb := mustBuffer(t, w, 1)
	for x := range w {
		fb.Set(x, 0, engine.Cell{Character: '.', Foreground: red, Background: green})
	}
	return fb
}

func draw(t *testing.T, fb *engine.FrameBuffer, sprites ...*Sprite) {
	t.Helper()
	l := NewLayer()
	for _, sp := range sprites {
		l.Add(sp)
	}
	l.Draw(fb)
}

// Holes of the art let the floor show through, untouched, dot and color; the letters
// keep the floor's background where they have none of their own.
func TestOverlayHolesShowTheFloorAndLettersKeepItsBackground(t *testing.T) {
	letters := FromLines(engine.Cell{Foreground: red}, "a b")
	fb := floor(t, 3)
	draw(t, fb, &Sprite{Frame: letters, Overlay: true})

	wantA := engine.Cell{Character: 'a', Foreground: red, Background: green}
	hole := engine.Cell{Character: '.', Foreground: red, Background: green}
	wantB := engine.Cell{Character: 'b', Foreground: red, Background: green}
	for x, want := range []engine.Cell{wantA, hole, wantB} {
		if got := fb.At(x, 0); got != want {
			t.Errorf("cell %d: wanted %+v, got %+v", x, want, got)
		}
	}

	// Without Overlay the same sprite stamps over the floor: the hole erases the dot, and
	// the letters have the default background.
	fb = floor(t, 3)
	draw(t, fb, &Sprite{Frame: letters})
	if got := fb.At(1, 0); got != engine.BlankCell {
		t.Errorf("wanted the hole to erase the floor without Overlay, got %+v", got)
	}
	if got := fb.At(0, 0); got.Background != (engine.Color{}) {
		t.Errorf("wanted the letter to have the default background without Overlay, got %+v", got)
	}
}

// A cell with a background of its own keeps it, and a blank that has one is a colored
// block, not a hole.
func TestOverlayCellsWithTheirOwnBackgroundKeepIt(t *testing.T) {
	blue := engine.Palette(21)
	art := NewFrame(2, 1)
	art.Set(0, 0, engine.Cell{Character: 'a', Background: blue})
	art.Set(1, 0, engine.Cell{Background: blue}) // a blank with a color
	fb := floor(t, 2)
	draw(t, fb, &Sprite{Frame: art, Overlay: true})

	if got := fb.At(0, 0); got.Character != 'a' || got.Background != blue {
		t.Errorf("wanted the letter on its own background, got %+v", got)
	}
	if got := fb.At(1, 0); got.Background != blue || got.Character != 0 {
		t.Errorf("wanted a colored blank to be drawn, not skipped, got %+v", got)
	}
}

// What shows by itself is not a hole, whatever the character: a blank with an attribute
// (Reverse swaps the colors, Underline draws a line) is drawn, and so is any attribute.
func TestOverlayBlanksWithAttributesAreNotHoles(t *testing.T) {
	for _, attr := range []engine.Attr{engine.Reverse, engine.Underline, engine.Bold, engine.Dim} {
		fb := floor(t, 1)
		draw(t, fb, &Sprite{Cell: engine.Cell{Character: ' ', Attrs: attr}, Overlay: true})
		if got := fb.At(0, 0); got.Attrs != attr || got.Character != ' ' || got.Background != green {
			t.Errorf("attr %v: wanted the blank drawn with its attribute on the floor's background, got %+v", attr, got)
		}
	}
}

// A blank is a space or NUL, with a foreground or not: the foreground of a blank is not seen.
func TestOverlayBlankCharactersAreHolesWhateverTheirForeground(t *testing.T) {
	for _, c := range []engine.Cell{{}, {Character: ' '}, {Foreground: red}, {Character: ' ', Foreground: red}} {
		fb := floor(t, 1)
		draw(t, fb, &Sprite{Cell: c, Overlay: true})
		if got := fb.At(0, 0); got != (engine.Cell{Character: '.', Foreground: red, Background: green}) {
			t.Errorf("cell %+v: wanted a hole that leaves the floor alone, got %+v", c, got)
		}
	}
}

// The single Cell of a sprite is see-through the same way.
func TestOverlayAppliesToASingleCell(t *testing.T) {
	fb := floor(t, 2)
	draw(t, fb,
		&Sprite{Position: geometry.Vector2{X: 0}, Cell: engine.Cell{Character: 'x', Foreground: red}, Overlay: true},
		&Sprite{Position: geometry.Vector2{X: 1}, Cell: engine.Cell{Character: ' '}, Overlay: true})

	if got := fb.At(0, 0); got != (engine.Cell{Character: 'x', Foreground: red, Background: green}) {
		t.Errorf("wanted the letter on the floor's background, got %+v", got)
	}
	if got := fb.At(1, 0); got.Character != '.' {
		t.Errorf("wanted the blank to leave the floor alone, got %+v", got)
	}
}

// Sprites overlay one another in order: a later overlay sprite keeps the background that
// an earlier one left, and a hole of the later one shows the earlier one.
func TestOverlaysStack(t *testing.T) {
	fb := mustBuffer(t, 2, 1)
	first := &Sprite{Cell: engine.Cell{Character: 'a', Background: green}, Overlay: true}
	second := &Sprite{Frame: FromLines(engine.Cell{Foreground: red}, " b"), Overlay: true}
	draw(t, fb, first, second)

	if got := fb.At(0, 0); got.Character != 'a' || got.Background != green {
		t.Errorf("wanted the hole of the second sprite to show the first, got %+v", got)
	}
	if got := fb.At(1, 0); got.Character != 'b' || got.Background != (engine.Color{}) {
		t.Errorf("wanted b on the buffer's own (default) background, got %+v", got)
	}

	fb = mustBuffer(t, 1, 1)
	draw(t, fb, &Sprite{Cell: engine.Cell{Character: 'a', Background: green}}, &Sprite{Cell: engine.Cell{Character: 'b'}, Overlay: true})
	if got := fb.At(0, 0); got.Character != 'b' || got.Background != green {
		t.Errorf("wanted the second sprite's letter on the first one's background, got %+v", got)
	}
}

// On a cell that Canvas.Compose wrote, only the background is kept: the pixel above it, the
// foreground of a half block, is gone. This is why the documentation says not to mix them.
func TestOverlayOnComposedPixelsKeepsOnlyTheBackground(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	fb := mustBuffer(t, 1, 1)
	canvas := fb.Canvas()
	canvas.Set(0, 0, red)   // the top pixel
	canvas.Set(0, 1, green) // the bottom pixel
	canvas.Compose()

	draw(t, fb, &Sprite{Cell: engine.Cell{Character: 'x'}, Overlay: true})

	if got := fb.At(0, 0); got.Character != 'x' || got.Background != green || got.Foreground == red {
		t.Errorf("wanted x on the bottom pixel's color and the top pixel lost, got %+v", got)
	}
}

// Overlay does not change where a sprite is drawn or what is clipped: an off-screen
// overlay sprite draws nothing, and an empty frame draws nothing.
func TestOverlayClipsAndSkipsAsBefore(t *testing.T) {
	fb := floor(t, 3)
	draw(t, fb,
		&Sprite{Position: geometry.Vector2{X: -5}, Cell: engine.Cell{Character: 'x'}, Overlay: true},
		&Sprite{Position: geometry.Vector2{X: 7}, Frame: FromLines(engine.Cell{}, "xx"), Overlay: true},
		&Sprite{Position: geometry.Vector2{X: 0}, Frame: NewFrame(0, 0), Overlay: true})

	if got := fb.String(); got != "..." {
		t.Errorf("wanted the floor untouched, got %q", got)
	}
}
