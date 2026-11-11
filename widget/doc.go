// Package widget draws the pieces of a heads-up display in screen space: a label, a panel and
// a bar.
//
// A widget is a value with exported fields, drawn into an [engine.FrameBuffer] by its Draw
// method at a position in cells, with the origin at the top left, as for
// [engine.FrameBuffer.DrawBox]. What runs off the frame is cut off, and a size that is not
// positive draws nothing, however large or small the numbers are. Widgets go over the world,
// so draw the world first, through its camera, and the widgets after it at places that do
// not move with the camera:
//
//	level.Draw(fb, view)
//	sprites.DrawView(fb, view)
//	widget.Panel{Title: "Status"}.Draw(fb, 0, 0, 16, 4)
//	widget.Label{Text: "coins 3/12"}.Draw(fb, 2, 1, 12)
//	widget.Bar{Value: health}.Draw(fb, 2, 2, 12)
//
// # Style
//
// A style is an [engine.Cell] whose Foreground, Background and Attrs are used and whose
// Character is not. The zero Cell is the terminal's default colors and plain text.
//
// # Text
//
// A [Label], and the title of a [Panel], are counted in cells as [engine.TextWidth] counts
// them, so they line up with [engine.FrameBuffer.DrawText]: a combining mark takes no cell,
// and a wide character is one cell, drawn as '?'. Text that is wider than the room it has is
// cut off at the edge, and a combining mark stays with the character before it.
//
// # Holes
//
// A [Panel] fills its inside with Fill. A Fill that is blank, with no background and no
// attributes, as the zero Cell is, is a hole: the inside is left alone and shows what is
// under it, as a see-through sprite does.
package widget
