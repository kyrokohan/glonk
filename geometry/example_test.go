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
