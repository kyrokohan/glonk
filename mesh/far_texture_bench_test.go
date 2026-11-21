package mesh

import (
	"math"
	"testing"

	"glonk.dev/bitmap"
	"glonk.dev/engine"
	"glonk.dev/geometry"
)

// BenchmarkDrawTexturedFar is a floor with a picture of 64 by 64 going away into the distance, the
// case where the copy of the picture to read changes across one face, in cells and in pixels.
func BenchmarkDrawTexturedFar(b *testing.B) {
	pic := bitmap.New(64, 64)
	for y := range 64 {
		for x := range 64 {
			c := engine.RGB(230, 230, 230)
			if (x/4+y/4)%2 == 1 {
				c = engine.RGB(30, 30, 30)
			}
			pic.Set(x, y, c)
		}
	}
	floor := Plane(40, 40)
	floor.Material.Texture = pic
	floor.Transform.Position = geometry.Vector3{Y: -1, Z: 20}
	cam := Camera{FOV: math.Pi / 2}
	lights := []Light{Ambient{Level: 1}}
	for _, size := range []struct{ w, h int }{{80, 24}, {200, 60}} {
		fb, err := engine.NewFrameBuffer(size.w, size.h)
		if err != nil {
			b.Fatal(err)
		}
		cv := fb.Canvas()
		at := " " + dims(size.w, 0) + "x" + dims(size.h, 0)
		b.Run("cells"+at, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				fb.Clear()
				floor.DrawShaded(fb, cam, lights...)
			}
		})
		b.Run("pixels"+at, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				cv.Clear()
				floor.DrawShadedPixels(cv, cam, lights...)
			}
		})
	}
}
