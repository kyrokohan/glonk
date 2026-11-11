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
