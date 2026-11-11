package geometry_test

import (
	"fmt"
	"math"

	"glonk.dev/geometry"
)

func ExampleVector3_Cross() {
	right := geometry.Vector3{X: 1}
	up := geometry.Vector3{Y: 1}

	fmt.Println(right.Cross(up)) // perpendicular to both: straight ahead
	// Output:
	// {0 0 1}
}

func ExampleVector2_Rotate() {
	v := geometry.Vector2{X: 1}
	r := v.Rotate(math.Pi / 2)

	fmt.Printf("%.2f %.2f\n", r.X, r.Y)
	// Output:
	// 0.00 1.00
}

// Rectangles are half-open: one that ends where another starts only touches it.
func ExampleRect_Overlaps() {
	a := geometry.NewRect(0, 0, 4, 4)
	b := geometry.NewRect(3, 1, 4, 2)
	c := geometry.NewRect(4, 0, 2, 2)

	fmt.Println(a.Overlaps(b), a.Intersect(b))
	fmt.Println(a.Overlaps(c), a.Intersect(c)) // c starts where a ends
	// Output:
	// true {{3 1} {4 3}}
	// false {{0 0} {0 0}}
}

// PushOutOf says how far to move a box to stand it against another: the shortest way out.
func ExampleRect_PushOutOf() {
	wall := geometry.NewRect(10, 0, 2, 8) // a wall from x=10 to x=12
	player := geometry.NewRect(8.5, 3, 2, 2)

	push := player.PushOutOf(wall)
	fmt.Println(push) // back the way it came, by what it went into the wall
	player = player.Translate(push)
	fmt.Println(player.Overlaps(wall), player.Max.X)
	// Output:
	// {-0.5 0}
	// false 10
}

// A camera is the rectangle of the screen, centered on the player and kept inside the level.
func ExampleRect_Within() {
	screen := geometry.NewRect(0, 0, 80, 24)
	level := geometry.NewRect(0, 0, 200, 60)

	for _, player := range []geometry.Vector2{{X: 100, Y: 30}, {X: 5, Y: 5}} {
		view := screen.CenterOn(player).Within(level)
		fmt.Println(view.Min)
	}
	// Output:
	// {60 18}
	// {0 0}
}
