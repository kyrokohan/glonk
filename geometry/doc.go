// Package geometry provides 2D and 3D vector math, and 2D rectangles.
//
// Angles are in radians. In 3D, x points right, y up and z away from the viewer,
// which is how the mesh package places and draws things. RotateX, RotateY and
// RotateZ say which way a positive angle turns.
//
// A [Rect] is an axis-aligned box: the points from Min up to, but not including, Max, like
// the cells a sprite covers, so rectangles that only touch do not overlap. [Rect.Overlaps]
// and [Rect.Intersect] ask what two boxes share, and [Rect.PushOutOf] gives the smallest
// move that takes one out of another, which is what a wall or a paddle needs. A rectangle
// with no area, or a NaN in it, is empty and overlaps nothing.
//
// A rectangle is also a camera: the rectangle of the screen, in the world's cells, moved with
// [Rect.CenterOn] to center on the player and with [Rect.Within] to stay inside the level.
package geometry
