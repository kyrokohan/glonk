// Package widget draws the pieces of a heads-up display and a menu in screen space: a label,
// a panel, a bar and a menu.
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
// # Menu
//
// A [Menu] is the one widget that reacts to input. Call [Menu.Update] from the game's Update
// with its [engine.Input], and [Menu.Draw] from the game's Draw. Up and Down move the
// selection, wrapping around at the ends, and Enter chooses; Update returns the index of the
// item chosen. Escape and every other key are left to the game, so a game can open the menu
// with Escape and close it again. A list longer than its rows scrolls to keep the selected
// item in view. With [engine.Config.Mouse] set, the pointer selects the item it moves onto, a
// click chooses one, and the wheel steps through them, against where Draw last drew the menu.
// To test a game that has a menu, drive it with an [engine.Keyboard].
//
//	choice, chosen := g.menu.Update(in)
//	if chosen && choice == quitItem {
//		return engine.ErrQuit
//	}
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
//
// # A picture in a panel
//
// The pixels of an [engine.Canvas] are under every cell until [engine.Canvas.Compose] puts them
// down, so a picture drawn on the canvas is covered by the panel's fill. Draw the panel, then the
// picture, then put that rectangle of the canvas down over the cells with
// [engine.Canvas.ComposeRect]. A cell is one pixel across and two down, so pixel row 2 is cell
// row 1, and where the picture is see-through the fill shows:
//
//	widget.Panel{Title: "coins", Fill: fill}.Draw(fb, 0, 0, 20, 5)
//	canvas := fb.Canvas()
//	coin.Draw(canvas, 2, 2) // a bitmap.Bitmap, at pixel (2, 2): inside the panel
//	canvas.ComposeRect(2, 2, coin.Width(), coin.Height())
//	widget.Label{Text: "12"}.Draw(fb, 9, 2, 4) // beside it, in cells
package widget
