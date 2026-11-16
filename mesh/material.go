package mesh

import (
	"math"

	"glonk.dev/bitmap"
	"glonk.dev/engine"
)

// Material says how a mesh's surface looks when it is drawn shaded.
type Material struct {
	// Color is the color of the surface where the light meets it fully, and the
	// color the characters are drawn in. The zero Color is the terminal's default.
	// Mesh.FaceColors replaces it for the faces that have a color of their own.
	Color engine.Color

	// Texture is a picture for the faces that have Mesh.FaceTexCoords: they take the color
	// of the surface from it, as Color is for the others. Nil, or a picture with no pixels,
	// is none. A mesh has one picture; for two, make a mesh for each, or put both in one
	// picture and give each face the coordinates of its part. A view from Bitmap.Sub is a
	// picture of its own, and repeats inside itself. The colors are the ones Bitmap.At gives,
	// so a Shaded or Tinted view is drawn as the picture it is a view of. Drawing reads the
	// picture and does not copy it, so it is not to be changed by another goroutine while
	// the mesh is drawn.
	Texture *bitmap.Bitmap

	// Ramp is the characters that show brightness in DrawShaded, from the darkest to
	// the brightest. An empty Ramp means ".:-=+*#%@", plain ASCII that renders in any
	// terminal and has no space, so a lit face never vanishes into the background. A
	// ramp with one character fills every face with it. Characters follow the same
	// rules as everywhere in the engine: only one-column characters are drawn as they
	// are. DrawShadedPixels shows brightness by color and ignores Ramp.
	Ramp string
}

// defaultRamp is what an empty Material.Ramp stands for.
const defaultRamp = ".:-=+*#%@"

var defaultRampRunes = []rune(defaultRamp)

// ramp returns the characters to shade with, darkest first.
func (m Material) ramp() []rune {
	if m.Ramp == "" {
		return defaultRampRunes
	}
	return []rune(m.Ramp)
}

// shadeChar maps an intensity from 0 (darkest) to 1 (brightest) onto the ramp, which
// must not be empty. Out-of-range values clamp; NaN is treated as dark.
func shadeChar(ramp []rune, intensity float64) rune {
	if math.IsNaN(intensity) {
		intensity = 0
	}
	i := int(math.Round(min(max(intensity, 0), 1) * float64(len(ramp)-1)))
	return ramp[i]
}
