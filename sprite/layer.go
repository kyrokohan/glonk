package sprite

import (
	"math"
	"slices"

	"glonk.dev/engine"
)

// Layer is an ordered set of sprites drawn together; later sprites draw over earlier ones.
type Layer struct {
	sprites []*Sprite
}

// NewLayer returns an empty Layer.
func NewLayer() *Layer {
	return &Layer{}
}

// Add appends obj to the layer. A nil sprite is ignored.
func (l *Layer) Add(obj *Sprite) {
	if obj == nil {
		return
	}
	l.sprites = append(l.sprites, obj)
}

// Remove takes obj out of the layer, every time it was added, and reports whether it
// was there. The sprites left keep their order, so they still draw over one another
// as before. A nil sprite removes nothing.
func (l *Layer) Remove(obj *Sprite) bool {
	before := len(l.sprites) // a nil sprite is never in the list, so it finds nothing
	l.sprites = slices.DeleteFunc(l.sprites, func(sp *Sprite) bool { return sp == obj })
	return len(l.sprites) < before
}

// Clear takes every sprite out of the layer.
func (l *Layer) Clear() {
	clear(l.sprites) // so the old slots do not keep the sprites alive
	l.sprites = l.sprites[:0]
}

// Len returns how many sprites are in the layer. A sprite added twice counts twice.
func (l *Layer) Len() int {
	return len(l.sprites)
}

// Draw sets each sprite's Cell in fb, clipping anything off screen. A sprite replaces
// the whole cell under it. A sprite whose
// Position has a NaN or infinite X or Y is not drawn, like one that is off screen.
func (l *Layer) Draw(fb *engine.FrameBuffer) {
	w, h := float64(fb.Width()), float64(fb.Height())

	for _, sp := range l.sprites {
		x, y := math.Floor(sp.Position.X), math.Floor(sp.Position.Y)

		// Checked before converting to int, and written so that NaN fails: what
		// int(NaN) or int(1e300) gives differs from one CPU to the next.
		if !(x >= 0 && x < w && y >= 0 && y < h) {
			continue
		}
		fb.Set(int(x), int(y), sp.Cell)
	}
}
