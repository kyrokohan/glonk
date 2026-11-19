package mesh_test

import (
	"testing"

	"glonk.dev/engine"
	"glonk.dev/geometry"
	"glonk.dev/mesh"
)

var sink geometry.Vector3

// Every pixel's ray of a 240 by 134 pixel canvas, as RayPixels gives it.
func BenchmarkRayPixelsEachPixel(b *testing.B) {
	fb, err := engine.NewFrameBuffer(240, 67)
	if err != nil {
		b.Fatal(err)
	}
	cv := fb.Canvas()
	cam := mesh.Camera{Yaw: 0.4, Pitch: -0.2}
	b.ReportAllocs()
	for b.Loop() {
		for y := range cv.Height() {
			for x := range cv.Width() {
				_, d, _ := cam.RayPixels(float64(x)+0.5, float64(y)+0.5, cv)
				sink = sink.Add(d)
			}
		}
	}
}

// The same rays from RaysPixels: the first pixel's, and the steps across and down.
func BenchmarkRaysPixels(b *testing.B) {
	fb, err := engine.NewFrameBuffer(240, 67)
	if err != nil {
		b.Fatal(err)
	}
	cv := fb.Canvas()
	cam := mesh.Camera{Yaw: 0.4, Pitch: -0.2}
	b.ReportAllocs()
	for b.Loop() {
		rays, _ := cam.RaysPixels(cv)
		for y := range cv.Height() {
			row := rays.First.Add(rays.Down.Scale(float64(y)))
			for x := range cv.Width() {
				sink = sink.Add(row.Add(rays.Across.Scale(float64(x))))
			}
		}
	}
}
