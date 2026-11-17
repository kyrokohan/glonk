package mesh

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// lamp is a light of a program's own that says how far it reaches, and gives nothing beyond.
type lamp struct {
	at    geometry.Vector3
	reach float64
}

func (l lamp) Brightness(p, n geometry.Vector3) float64 {
	d := p.Sub(l.at).Length()
	if !(d < l.reach) {
		return 0
	}
	return (1 - d/l.reach) * clamp01(n.Dot(l.at.Sub(p).Normalize()))
}

func (l lamp) Reach() (geometry.Vector3, float64) { return l.at, l.reach }

// countingLamp is a lamp that counts the times it is asked.
type countingLamp struct {
	lamp
	asked int
}

func (l *countingLamp) Brightness(p, n geometry.Vector3) float64 {
	l.asked++
	return l.lamp.Brightness(p, n)
}

func (l *countingLamp) Reach() (geometry.Vector3, float64) { return l.lamp.Reach() }

// plainLamp is a lamp that does not say how far it reaches (it does not embed lamp, whose
// Reach it would have), and counts the times it is asked.
type plainLamp struct {
	at    geometry.Vector3
	reach float64
	asked int
}

func (l *plainLamp) Brightness(p, n geometry.Vector3) float64 {
	l.asked++
	return lamp{l.at, l.reach}.Brightness(p, n)
}

func TestPointSaysHowFarItReaches(t *testing.T) {
	at := vec(1, 2, 3)
	nan := math.NaN()
	for name, tt := range map[string]struct {
		rng  float64
		want float64
	}{
		"a range":    {8, 8},
		"no fading":  {0, math.Inf(1)},
		"negative":   {-1, 0},
		"not a num":  {nan, 0},
		"minus zero": {math.Copysign(0, -1), math.Inf(1)},
	} {
		c, r := Point{Position: at, Range: tt.rng}.Reach()
		if c != at || r != tt.want {
			t.Errorf("%s: wanted %v and %v, got %v and %v", name, at, tt.want, c, r)
		}
	}
	var _ reacher = Point{}
	var _ reacher = &Point{}
}

func TestCannotReach(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	o := geometry.Vector3{}
	for name, tt := range map[string]struct {
		lc   geometry.Vector3
		lr   float64
		mc   geometry.Vector3
		mr   float64
		want bool
	}{
		"far apart":                     {vec(10, 0, 0), 2, o, 3, true},
		"just apart":                    {vec(5.001, 0, 0), 2, o, 3, true},
		"touching":                      {vec(5, 0, 0), 2, o, 3, false},
		"overlapping":                   {vec(4, 0, 0), 2, o, 3, false},
		"rounding of the sums":          {vec(5+1e-12, 0, 0), 2, o, 3, false},
		"inside the mesh":               {o, 1, o, 3, false},
		"no radius, inside":             {o, 0, o, 3, false},
		"no radius, outside":            {vec(3.5, 0, 0), 0, o, 3, true},
		"a lamp that reaches all":       {vec(1e9, 0, 0), inf, o, 3, false},
		"a mesh that is infinite":       {vec(1e9, 0, 0), 2, o, inf, false},
		"lamp radius NaN":               {vec(10, 0, 0), nan, o, 3, false},
		"mesh radius NaN":               {vec(10, 0, 0), 2, o, nan, false},
		"lamp radius negative":          {vec(10, 0, 0), -2, o, 3, false},
		"mesh radius negative":          {vec(10, 0, 0), 2, o, -3, false},
		"lamp place NaN":                {vec(nan, 0, 0), 2, o, 3, false},
		"mesh place NaN":                {vec(10, 0, 0), 2, vec(0, nan, 0), 3, false},
		"far from the origin, apart":    {vec(1e6+5.01, 0, 0), 2, vec(1e6, 0, 0), 3, true},
		"far from the origin, touching": {vec(1e6+5, 0, 0), 2, vec(1e6, 0, 0), 3, false},
		"rounded at 1e9, touching":      {vec(1e9+5+1.2e-7, 0, 0), 2, vec(1e9, 0, 0), 3, false}, // 1.2e-7 is a place's own rounding there
		"rounded at 1e9, apart":         {vec(1e9+50, 0, 0), 2, vec(1e9, 0, 0), 3, true},
		"diagonal":                      {vec(3, 4, 12), 5, o, 7.9, true}, // 13 apart, 12.9 reached
		"diagonal, touching":            {vec(3, 4, 12), 5, o, 8, false},
	} {
		if got := cannotReach(tt.lc, tt.lr, tt.mc, tt.mr); got != tt.want {
			t.Errorf("%s: wanted %v, got %v", name, tt.want, got)
		}
	}
}

// lightsReaching keeps the order, drops only the lights that cannot reach, and keeps the rest:
// those that do not say how far they reach and the nil one.
func TestLightsReachingKeepsTheRightOnes(t *testing.T) {
	ball := Sphere(1, 12, 6)
	ball.Transform.Position = vec(10, 0, 0)
	near, far := lamp{vec(12, 0, 0), 3}, lamp{vec(30, 0, 0), 3}
	plain := &plainLamp{at: far.at, reach: far.reach}
	lights := []Light{Ambient{Level: 0.2}, Point{Position: far.at, Range: 3}, Point{Position: near.at, Range: 3}, nil, far, near, plain, Directional{Direction: vec(0, -1, 0)}}
	want := []Light{lights[0], lights[2], nil, near, plain, lights[7]}

	s := getScratch(0)
	defer s.release()
	got := s.lightsReaching(&ball, ball.Transform.placement(), lights)
	if !slices.Equal(got, want) {
		t.Errorf("wanted %v, got %v", want, got)
	}
	if !slices.Equal(lights[:3], []Light{Ambient{Level: 0.2}, Point{Position: far.at, Range: 3}, Point{Position: near.at, Range: 3}}) {
		t.Errorf("wanted the caller's lights left alone, got %v", lights)
	}

	// With the switch off, or a mesh without bounds, every light stays.
	for name, tt := range map[string]struct {
		m   Mesh
		off bool
	}{"culling off": {ball, true}, "no bounds": {func() Mesh { m := ball; m.bounds = nil; return m }(), false}} {
		cullingOn = !tt.off
		got := s.lightsReaching(&tt.m, tt.m.Transform.placement(), lights)
		cullingOn = true
		if !slices.Equal(got, lights) {
			t.Errorf("%s: wanted every light, got %v", name, got)
		}
	}

	// All of them can go.
	got = s.lightsReaching(&ball, ball.Transform.placement(), []Light{Point{Position: far.at, Range: 3}, far})
	if len(got) != 0 {
		t.Errorf("wanted none, got %v", got)
	}
	if got = s.lightsReaching(&ball, ball.Transform.placement(), nil); len(got) != 0 {
		t.Errorf("wanted none for no lights, got %v", got)
	}
}

// Whether a light is asked follows its ball and the mesh's, to the edge.
func TestALightThatCannotReachAMeshIsNotAsked(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	ball := Sphere(1, 24, 12)
	ball.Transform.Position = vec(0, 0, 5)
	cam := shadeCam()
	at := func(x float64) lamp { return lamp{vec(x, 0, 5), 2} }
	for name, tt := range map[string]struct {
		lamp    lamp
		asked   bool
		noReach bool
	}{
		"far to the side":        {at(30), false, false},
		"just beyond the ball":   {at(3.001), false, false},
		"touching the ball":      {at(2.999), true, false},
		"beside the mesh":        {at(1.5), true, false},
		"in the mesh":            {at(0), true, false},
		"far, but without Reach": {at(30), true, true},
	} {
		for _, pixels := range []bool{false, true} {
			var asked int
			var light Light
			if tt.noReach {
				l := &plainLamp{at: tt.lamp.at, reach: tt.lamp.reach}
				light, asked = l, 0
				draw(t, ball, cam, pixels, light)
				asked = l.asked
			} else {
				l := &countingLamp{lamp: tt.lamp}
				light = l
				draw(t, ball, cam, pixels, light)
				asked = l.asked
			}
			if (asked > 0) != tt.asked {
				t.Errorf("%s (pixels %v): wanted asked %v, asked %d times", name, pixels, tt.asked, asked)
			}
		}
	}

	// A mesh whose bounds are not current asks every light, as before.
	l := &countingLamp{lamp: at(30)}
	stale := ball
	stale.Vertices = slices.Clone(stale.Vertices)
	draw(t, stale, cam, false, l)
	if l.asked == 0 {
		t.Errorf("wanted a mesh without current bounds to ask a far light")
	}
}

func draw(t *testing.T, m Mesh, cam Camera, pixels bool, lights ...Light) {
	t.Helper()
	fb := mustBuffer(t, 40, 16)
	if pixels {
		canvas := fb.Canvas()
		m.DrawShadedPixels(canvas, cam, lights...)
		return
	}
	m.DrawShaded(fb, cam, lights...)
}

// A flat mesh asks about the middle of each face, which is also inside its ball.
func TestAFlatMeshDoesNotAskALightThatCannotReachIt(t *testing.T) {
	m := Cube(2)
	m.Transform.Position = vec(0, 0, 5)
	m.Normals = nil
	far, near := &countingLamp{lamp: lamp{vec(40, 0, 5), 3}}, &countingLamp{lamp: lamp{vec(3, 1, 5), 3}}
	draw(t, m, shadeCam(), false, far, near)
	if far.asked != 0 || near.asked == 0 {
		t.Errorf("wanted the far lamp not asked and the near one asked, got %d and %d", far.asked, near.asked)
	}
}

// Leaving lights out changes no picture: scenes of many lamps, drawn with skipping on and off, in
// every way a mesh is lit, are the same pictures.
func TestLeavingLightsOutChangesNoPicture(t *testing.T) {
	t.Setenv("TERM_PROGRAM", "")
	defer func(on bool) { cullingOn = on }(cullingOn)
	rng := rand.New(rand.NewPCG(71, 72))
	nan := math.NaN()
	left, kept, lit := 0, 0, 0

	for scene := range 500 {
		var m Mesh
		switch scene % 5 {
		case 0:
			m = Sphere(0.5+2*rng.Float64(), 4+rng.IntN(14), 3+rng.IntN(8))
		case 1:
			m = Cube(0.5 + 3*rng.Float64()).SmoothNormals()
		case 2:
			m = Cylinder(0.5+rng.Float64(), 1+2*rng.Float64(), 3+rng.IntN(12))
		case 3:
			m = Plane(2+8*rng.Float64(), 2+8*rng.Float64())
			m.Transform.Pitch = -0.6 // tipped toward the camera
		default:
			m = Cone(0.5+rng.Float64(), 1+3*rng.Float64(), 3+rng.IntN(12))
		}
		if scene%3 == 1 {
			m.Normals = nil // lit face by face
		}
		if scene%4 == 2 {
			m.VertexColors = make([]engine.Color, len(m.Vertices))
			for i := range m.VertexColors {
				m.VertexColors[i] = engine.RGB(uint8(rng.IntN(256)), uint8(rng.IntN(256)), uint8(rng.IntN(256)))
			}
		}
		// Written at the place it is, with no transform, or placed; the second is sometimes turned and scaled.
		far := 0.0
		if scene%7 == 3 {
			far = 1e5 // where a rounding is the size of a ten-thousandth of a cell
		}
		cam := Camera{Position: vec(far, far, far), FOV: 1 + rng.Float64(), Near: 0.1 + rng.Float64()}
		spot := vec(far+rng.Float64()*8-4, far+rng.Float64()*4-2, far+3+rng.Float64()*6)
		if rng.IntN(2) == 0 {
			m = atTheWorld(m, spot)
			m.UpdateBounds()
		} else {
			m.Transform.Position = spot
			m.Transform.Yaw = rng.Float64() * 6
			if rng.IntN(2) == 0 {
				m.Transform.Scale = []float64{0.3, 1, 2.5}[rng.IntN(3)]
			}
		}
		if rng.IntN(3) == 0 {
			cam.Fog = &Fog{Color: engine.Palette(8), Start: 1, End: 30}
		}
		m.DoubleSided = rng.IntN(3) == 0

		var lights []Light
		for range 2 + rng.IntN(14) {
			at := vec(far+rng.Float64()*40-20, far+rng.Float64()*16-8, far+rng.Float64()*40-8)
			reach := 0.5 + 12*rng.Float64()
			switch rng.IntN(8) {
			case 0:
				lights = append(lights, lamp{at, reach})
			case 1:
				lights = append(lights, Point{Position: at}) // no fading: reaches everywhere
			case 2:
				lights = append(lights, Point{Position: at, Range: []float64{-1, nan}[rng.IntN(2)]})
			case 3:
				lights = append(lights, []Light{nil, Ambient{Level: 0.2}, Directional{Direction: vec(0.4, -1, 0.6)}}[rng.IntN(3)])
			default:
				lights = append(lights, Point{Position: at, Range: reach, Intensity: 0.3 + 0.7*rng.Float64()})
			}
		}
		w, h := 12+rng.IntN(70), 6+rng.IntN(24)

		drawAll := func() [4]string {
			glyphs, pixels, flatGlyphs, flatPixels := mustBuffer(t, w, h), mustBuffer(t, w, h), mustBuffer(t, w, h), mustBuffer(t, w, h)
			colored := m.withColor(engine.Palette(33))
			colored.DrawShaded(glyphs, cam, lights...)
			canvas := pixels.Canvas()
			colored.DrawShadedPixels(canvas, cam, lights...)
			canvas.Compose()
			flat := colored.Clone()
			flat.Normals = nil
			flat.DrawShaded(flatGlyphs, cam, lights...)
			canvas = flatPixels.Canvas()
			flat.DrawShadedPixels(canvas, cam, lights...)
			canvas.Compose()
			return [4]string{picture(glyphs), picture(pixels), picture(flatGlyphs), picture(flatPixels)}
		}
		cullingOn = true
		with := drawAll()
		cullingOn = false
		without := drawAll()
		cullingOn = true
		if without[0] != picture(mustBuffer(t, w, h)) {
			lit++
		}
		for i, name := range []string{"DrawShaded", "DrawShadedPixels", "flat DrawShaded", "flat DrawShadedPixels"} {
			if with[i] != without[i] {
				t.Fatalf("scene %d (%dx%d): wanted %s to draw the same with and without leaving lights out, got\n%s\nwanted\n%s", scene, w, h, name, with[i], without[i])
			}
		}

		s := getScratch(0)
		n := len(s.lightsReaching(&m, m.Transform.placement(), lights))
		s.release()
		left, kept = left+len(lights)-n, kept+n
	}
	// Plenty are left out, and plenty are kept, so that neither half of the test is vacuous.
	if left < 1500 || kept < 1000 || lit < 250 {
		t.Errorf("wanted plenty of lights left out and kept, and scenes with something drawn, got %d, %d and %d", left, kept, lit)
	}
}

func TestDrawingWithLightsLeftOutAllocatesNothingOnceWarm(t *testing.T) {
	if raceEnabled {
		t.Skip("the race detector makes sync.Pool drop items, so allocation counts mean nothing")
	}
	t.Setenv("TERM_PROGRAM", "")
	m := Sphere(1.6, 10, 6)
	m.Transform = Transform{Position: vec(0, 0, 4)}
	var lights []Light
	for i := range 20 {
		lights = append(lights, Point{Position: vec(float64(i*7), 0, 4), Range: 3})
	}
	lights = append(lights, lamp{vec(60, 0, 4), 3}, Ambient{Level: 0.1})
	fb := newShadeBuffer(t)
	for name, draw := range map[string]func(){
		"glyphs": func() { m.DrawShaded(fb, shadeCam(), lights...) },
		"pixels": func() { m.DrawShadedPixels(fb.Canvas(), shadeCam(), lights...) },
	} {
		draw()
		if n := testing.AllocsPerRun(20, draw); n != 0 {
			t.Errorf("%s: wanted no allocation, got %v", name, n)
		}
	}
}

// The pool must not keep a program's lights alive.
func TestReleasingAScratchForgetsTheLights(t *testing.T) {
	kept := []Light{Ambient{Level: 0.5}, Point{Range: 1}}
	s := &scratch{lit: kept}
	s.release()
	if kept[0] != nil || kept[1] != nil {
		t.Errorf("wanted the lights forgotten, got %v", kept)
	}
	// And the ones taken out by lightsReaching, which are past the end of what it returns.
	m := Sphere(1, 8, 4)
	s = getScratch(0)
	out := s.lightsReaching(&m, m.Transform.placement(), []Light{Point{Position: vec(50, 0, 0), Range: 1}, Ambient{Level: 0.5}, Point{Position: vec(60, 0, 0), Range: 1}})
	if len(out) != 1 {
		t.Fatalf("wanted one light kept, got %v", out)
	}
	if tail := out[:cap(out)]; len(tail) > 1 && (tail[1] != nil || (len(tail) > 2 && tail[2] != nil)) {
		t.Errorf("wanted the places of the lights left out cleared, got %v", tail)
	}
	s.release()
	if out[0] != nil {
		t.Errorf("wanted the light that was kept forgotten when the scratch is released, got %v", out[0])
	}

	// The same where nothing is left out.
	s = getScratch(0)
	all := s.lightsReaching(&m, m.Transform.placement(), []Light{Ambient{Level: 0.5}, Directional{Direction: vec(0, -1, 0)}})
	s.release()
	if all[0] != nil || all[1] != nil {
		t.Errorf("wanted the lights forgotten when nothing was left out, got %v", all)
	}
}
