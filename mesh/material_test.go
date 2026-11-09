package mesh

import (
	"math"
	"strings"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

func TestShadeCharWithOtherRamps(t *testing.T) {
	tests := []struct {
		name      string
		ramp      []rune
		intensity float64
		want      rune
	}{
		{"two characters, dark", []rune("ab"), 0.49, 'a'},
		{"two characters, bright", []rune("ab"), 0.5, 'b'},
		{"one character, dark", []rune("x"), 0, 'x'},
		{"one character, bright", []rune("x"), 1, 'x'},
		{"one character, NaN", []rune("x"), math.NaN(), 'x'},
		{"multi-byte characters", []rune(" ░▒▓█"), 1, '█'},
		{"multi-byte characters, middle", []rune(" ░▒▓█"), 0.5, '▒'},
		{"NaN is the darkest", []rune("abc"), math.NaN(), 'a'},
		{"above range is the brightest", []rune("abc"), 9, 'c'},
		{"below range is the darkest", []rune("abc"), -9, 'a'},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shadeChar(tt.ramp, tt.intensity); got != tt.want {
				t.Errorf("wanted %q, got %q", tt.want, got)
			}
		})
	}
}

func TestMaterialRamp(t *testing.T) {
	tests := []struct {
		name string
		ramp string
		want string
	}{
		{"empty is the default", "", defaultRamp},
		{"custom", " .oO", " .oO"},
		{"multi-byte", "░▒▓", "░▒▓"},
		{"invalid UTF-8 becomes the replacement character", "a\xffb", "a�b"},
		{"one character", "#", "#"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(Material{Ramp: tt.ramp}.ramp()); got != tt.want {
				t.Errorf("wanted %q, got %q", tt.want, got)
			}
		})
	}
}

// shadedChars draws a lit cube and returns the distinct characters it is made of.
func shadedChars(t *testing.T, m Mesh) map[rune]bool {
	t.Helper()
	fb := newShadeBuffer(t)
	m.DrawShaded(fb, shadeCam(), goldenSun)
	seen := map[rune]bool{}
	for _, c := range drawn(fb) {
		seen[fb.At(c[0], c[1]).Character] = true
	}
	return seen
}

func turnedCube() Mesh {
	cube := Cube(2)
	cube.Transform = Transform{Position: geometry.Vector3{Z: 5}, Yaw: 0.6, Pitch: -0.4}
	return cube
}

func TestDrawShadedUsesTheMaterialRamp(t *testing.T) {
	cube := turnedCube()
	cube.Material.Ramp = "123456789"

	seen := shadedChars(t, cube)

	if len(seen) < 2 {
		t.Fatalf("wanted at least two brightnesses on a turned cube, got %v", seen)
	}
	for r := range seen {
		if !strings.ContainsRune("123456789", r) {
			t.Errorf("wanted only characters of the ramp, got %q", r)
		}
	}
}

func TestDrawShadedWithAOneCharacterRamp(t *testing.T) {
	cube := turnedCube()
	cube.Material.Ramp = "#"

	seen := shadedChars(t, cube)

	if len(seen) != 1 || !seen['#'] {
		t.Errorf("wanted nothing but '#', got %v", seen)
	}
}

func TestDrawShadedWithTheDefaultRamp(t *testing.T) {
	seen := shadedChars(t, turnedCube())

	for r := range seen {
		if !strings.ContainsRune(defaultRamp, r) {
			t.Errorf("wanted only characters of the default ramp, got %q", r)
		}
	}
}

func TestMaterialColorIsWhatTheFacesAreDrawnIn(t *testing.T) {
	cube := turnedCube()
	cube.Material.Color = engine.Palette(33)
	fb := newShadeBuffer(t)

	cube.DrawShaded(fb, shadeCam(), goldenSun)

	cells := drawn(fb)
	if len(cells) == 0 {
		t.Fatalf("wanted something drawn")
	}
	for _, c := range cells {
		if got := fb.At(c[0], c[1]).Foreground; got != engine.Palette(33) {
			t.Fatalf("wanted palette(33) at %v, got %v", c, got)
		}
	}
}

// The ramp is for characters; shading with pixels goes by color and takes nothing from it.
func TestDrawShadedPixelsIgnoresTheRamp(t *testing.T) {
	plain, ramped := turnedCube().withColor(engine.Palette(208)), turnedCube().withColor(engine.Palette(208))
	ramped.Material.Ramp = "abc"
	_, a := newShadeCanvas(t)
	_, b := newShadeCanvas(t)

	plain.DrawShadedPixels(a, shadeCam(), goldenSun)
	ramped.DrawShadedPixels(b, shadeCam(), goldenSun)

	if len(pixelsDrawn(a)) == 0 {
		t.Fatalf("wanted pixels drawn")
	}
	for y := range a.Height() {
		for x := range a.Width() {
			if a.At(x, y) != b.At(x, y) {
				t.Fatalf("wanted the same pixels with and without a ramp, differ at (%d,%d)", x, y)
			}
		}
	}
}

func TestCloneAndFitKeepTheMaterial(t *testing.T) {
	m := Cube(2)
	m.Material = Material{Color: engine.Palette(9), Ramp: ".oO"}

	if got := m.Clone().Material; got != m.Material {
		t.Errorf("Clone: wanted %+v, got %+v", m.Material, got)
	}
	if got := m.Fit(5).Material; got != m.Material {
		t.Errorf("Fit: wanted %+v, got %+v", m.Material, got)
	}
}
