// Package engine runs a terminal render loop: a Game drawn into a FrameBuffer of
// cells, with keyboard Input.
//
// A program implements [Game] and gives it to [Run]. Run owns the terminal: it
// switches to the alternate screen, reads the keyboard into an [Input], calls Update
// and then Draw at a fixed frame rate, and puts the terminal back when it returns.
// To test a game, call Update with the Input of a [Keyboard] and read the frame as
// text with [FrameBuffer.String]; given an input and output that are not terminals,
// Run runs headless, which tests a whole run.
//
// Draw writes [Cell] values into a [FrameBuffer], a grid with its origin at the top left,
// x growing to the right and y growing down. Each cell is one character with a
// foreground and a background [Color] (the terminal's default, one of the 256
// palette colors, or RGB) and text attributes such as [Bold]. RGB is shown exactly on
// terminals with true color and as the nearest palette color on the rest, so output
// works in nearly any terminal. A [Canvas] gives the same frame a second
// layer of pixels, two to a cell, drawn with half-block characters; triangles, lines,
// rectangles and circles can be drawn into it, and the bitmap package draws pictures.
package engine
