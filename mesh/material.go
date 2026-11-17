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
	// of the surface from it, as Color is for the others. A pixel of it that is see-through
	// (a pixel of a bitmap.New picture that was never set, or one with an alpha under 128 in a
	// picture that was decoded) is a hole: see Mesh.FaceTexCoords. Nil, or a picture with no
	// pixels, is none. A mesh has one picture; for two, make a mesh for each, or put both in one
	// picture and give each face the coordinates of its part. A view from Bitmap.Sub is a
	// picture of its own, and repeats inside itself. The colors are the ones Bitmap.At gives,
	// so a Shaded or Tinted view is drawn as the picture it is a view of. Drawing reads the
	// picture and does not copy it, so it is not to be changed by another goroutine while
	// the mesh is drawn.
	Texture *bitmap.Bitmap

	// Opacity is the part of the surface's color that shows, from 0 to 1, for a mesh that can be
	// seen through: glass, water, smoke, a ghost. Each cell or pixel of one of its faces is mixed
	// over what is already there, as engine.Mix does, by this part: Opacity 0.25 is a quarter of
	// its color and three quarters of what is behind it. The color is lit and faded in fog first,
	// and is the picture's or the vertex colors' where it has them; a hole in a picture is still a
	// hole. 0, the default, means 1, as Light.Intensity: the mesh is solid. So does anything
	// that is not between 0 and 1, NaN included.
	//
	// Such a face is tested against depth, so a solid in front of it hides it, but does not store
	// it, so a see-through face behind it is still seen: draw the solid meshes first, and then the
	// see-through ones from far to near. glonk does not sort, between meshes or the faces of one: they
	// are mixed in the order they are drawn, the faces in the order of Mesh.Faces. A face seen from
	// behind is skipped unless Mesh.DoubleSided, as a solid's is, and then a closed shape such as a
	// glass ball is mixed in face order, so list the faces of its far side first or make two meshes.
	//
	// Over a pixel that is empty, or a cell that has nothing in it, the color is mixed with black,
	// as engine.Mix and engine.Shade read the default color, so a ghost shows against the empty
	// background; fill the canvas first to have another. In half-block pixels the pixel is mixed. In
	// cells the character and attributes of the cell behind stay, and its foreground and its
	// background are each mixed, so glass over a blank cell is a block of its color.
	Opacity float64

	// Ramp is the characters that show brightness in DrawShaded, from the darkest to
	// the brightest. An empty Ramp means ".:-=+*#%@", plain ASCII that renders in any
	// terminal and has no space, so a lit face never vanishes into the background. A
	// ramp with one character fills every face with it. Characters follow the same
	// rules as everywhere in the engine: only one-column characters are drawn as they
	// are. DrawShadedPixels shows brightness by color and ignores Ramp.
	Ramp string
}

// seeThrough returns the part of the color that shows, if the material can be seen through, and 0
// for a solid one: see Opacity.
func (m Material) seeThrough() float64 {
	if o := m.Opacity; o > 0 && o < 1 {
		return o
	}
	return 0
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
