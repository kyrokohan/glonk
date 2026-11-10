package engine

import (
	"fmt"
	"math"
)

// colorKind says what a Color holds.
type colorKind uint8

const (
	colorDefault colorKind = iota // the terminal's own color; the zero value
	colorPalette                  // one of the 256 palette colors
	colorRGB                      // an exact red, green and blue
)

// Color is a terminal color: the terminal's default, one of the 256 palette colors,
// or an RGB color. The zero Color is the default, so a Cell or a Config that leaves a
// color out gets the terminal's own. Make the others with Palette and RGB.
//
// Colors compare with ==. An RGB color is shown exactly on a terminal with true color
// (see Config.Colors) and as the closest of the palette colors 16 to 255 on one without.
type Color struct {
	// bits holds the kind in the top byte, and below it the palette index, or the red,
	// green and blue. One word, so that comparing two colors is one comparison, which
	// matters when every cell of a frame is compared with the one before it.
	bits uint32
}

// kind says what c holds.
func (c Color) kind() colorKind { return colorKind(c.bits >> 24) }

// Palette returns the palette color with the given index: 0 to 7 are the standard
// colors, 8 to 15 their bright versions, 16 to 231 a 6x6x6 color cube, and 232 to 255
// a gray ramp. Colors 0 to 15 are whatever the user's theme says; the rest are fixed.
func Palette(index uint8) Color {
	return Color{bits: uint32(colorPalette)<<24 | uint32(index)}
}

// RGB returns the color with the given red, green and blue, each 0 to 255. A
// terminal with true color shows it exactly; one without shows the nearest of the
// palette colors 16 to 255 (see Config.Colors).
func RGB(r, g, b uint8) Color {
	return Color{bits: uint32(colorRGB)<<24 | uint32(r)<<16 | uint32(g)<<8 | uint32(b)}
}

// IsDefault reports whether c is the terminal's default color.
func (c Color) IsDefault() bool {
	return c.kind() == colorDefault
}

// String describes c: "default", "palette(208)" or "#ff8700".
func (c Color) String() string {
	switch c.kind() {
	case colorPalette:
		return fmt.Sprintf("palette(%d)", uint8(c.bits))
	case colorRGB:
		return fmt.Sprintf("#%02x%02x%02x", uint8(c.bits>>16), uint8(c.bits>>8), uint8(c.bits))
	}
	return "default"
}

// RGB returns the red, green and blue of c, each 0 to 255, for working with colors, such
// as mixing two. An RGB color gives the numbers it was made from. A palette color is read
// as xterm defines it, whatever the user's theme makes of colors 0 to 15, and the default
// color, which is whatever the terminal likes, as black.
func (c Color) RGB() (r, g, b uint8) {
	switch c.kind() {
	case colorPalette:
		p := paletteRGB[uint8(c.bits)]
		return uint8(p[0]), uint8(p[1]), uint8(p[2])
	case colorRGB:
		return uint8(c.bits >> 16), uint8(c.bits >> 8), uint8(c.bits)
	}
	return 0, 0, 0
}

// paletteRGB is the red, green and blue of each of the 256 palette colors, as
// xterm defines them: 16 system colors, a 6x6x6 color cube, and 24 grays.
var paletteRGB = buildPalette()

func buildPalette() [256][3]int {
	var p [256][3]int

	// Colors 0 to 15 are whatever the user's theme says; these are xterm's defaults.
	system := [16][3]int{
		{0, 0, 0}, {205, 0, 0}, {0, 205, 0}, {205, 205, 0},
		{0, 0, 238}, {205, 0, 205}, {0, 205, 205}, {229, 229, 229},
		{127, 127, 127}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
		{92, 92, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
	}
	copy(p[:16], system[:])

	levels := [6]int{0, 95, 135, 175, 215, 255}
	for i := range 216 {
		p[16+i] = [3]int{levels[i/36], levels[(i/6)%6], levels[i%6]}
	}

	for i := range 24 {
		g := 8 + 10*i
		p[232+i] = [3]int{g, g, g}
	}
	return p
}

// cubeLevels are the six levels each channel takes in the palette's color cube.
var cubeLevels = [6]float64{0, 95, 135, 175, 215, 255}

// cubeLevel returns the index of the cube level nearest v, the lower one on a tie.
func cubeLevel(v float64) int {
	i := 0
	for _, midpoint := range [5]float64{47.5, 115, 155, 195, 235} {
		if v > midpoint {
			i++
		}
	}
	return i
}

// nearestPalette returns the palette color from 16 to 255 closest to the given red,
// green and blue (0 to 255, not necessarily whole numbers), and the lowest index on a
// tie. Colors 0 to 15 are left out because they depend on the user's theme.
//
// This does what a scan of all 240 colors would, in constant time: the nearest cube
// color takes the nearest level in each channel, the nearest gray is found from the
// average of the channels, and the closer of the two wins. An input that is a tie
// only up to rounding error may pick either of the two equally near colors.
func nearestPalette(r, g, b float64) uint8 {
	ri, gi, bi := cubeLevel(r), cubeLevel(g), cubeLevel(b)
	cube := 16 + 36*ri + 6*gi + bi
	cr, cg, cb := cubeLevels[ri], cubeLevels[gi], cubeLevels[bi]
	cubeDist := (r-cr)*(r-cr) + (g-cg)*(g-cg) + (b-cb)*(b-cb)

	// Gray number i is 8+10*i, and the sum of squared differences to it is smallest
	// where 3*(8+10*i) is nearest the sum of the channels.
	i := int(math.Ceil((r+g+b-24)/30 - 0.5))
	i = min(max(i, 0), 23)
	gray := float64(8 + 10*i)
	grayDist := (r-gray)*(r-gray) + (g-gray)*(g-gray) + (b-gray)*(b-gray)

	if grayDist < cubeDist {
		return uint8(232 + i)
	}
	return uint8(cube)
}

// Shade returns color scaled to the given brightness: 1 is color as it is, 0 is
// black. Intensity outside 0 to 1 is clamped and NaN is treated as 0.
//
// The result is an exact RGB color, whatever color is: a palette color is read as
// xterm defines it (colors 0 to 15 as xterm's defaults, though the user's theme may
// differ) and the default color as black. On a terminal with true color the shades
// run smoothly; on one without, each is shown as the nearest of the palette colors
// 16 to 255, as any RGB color is. Black comes out as RGB(0, 0, 0).
func Shade(color Color, intensity float64) Color {
	if math.IsNaN(intensity) {
		intensity = 0
	}
	intensity = min(max(intensity, 0), 1)

	r, g, b := color.RGB()
	scale := func(v uint8) uint8 { return uint8(math.Round(float64(v) * intensity)) }
	return RGB(scale(r), scale(g), scale(b))
}
