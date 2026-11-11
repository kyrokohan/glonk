// Package sprite draws sprites onto an engine.FrameBuffer in layers.
//
// A [Sprite] is a picture at a position: one [engine.Cell], a [Frame] of several cells,
// or an [Animation] of frames. Positions are in cells, with the origin at the top left,
// x to the right and y down; a fraction is floored, and for a frame the position is its
// top left cell. A sprite replaces the cells under it, background included, as
// [engine.FrameBuffer.Set] does, unless it is an overlay.
//
// # Frames
//
// [FromLines] makes a frame from text art: one cell per rune, short lines padded with
// blanks, and the style's colors and attributes on every rune that is not a space. [NewFrame]
// makes a blank grid to fill with [Frame.Set], for colors cell by cell. A frame that runs
// off an edge is cut off cell by cell.
//
//	ship := sprite.FromLines(engine.Cell{Foreground: engine.Palette(208)},
//		` /\ `,
//		`/__\`,
//	)
//	layer.Add(&sprite.Sprite{Position: geometry.Vector2{X: 2, Y: 1}, Frame: ship})
//
// Frames are shared by pointer, so two sprites with one frame see each other's changes;
// [Frame.Clone] makes a copy.
//
// # Overlay
//
// Off by default, [Sprite.Overlay] makes a sprite see-through. A blank cell of its art,
// with no background and no attributes, is a hole where what is behind shows, and any
// other cell that has no background of its own keeps the background of the cell under it,
// so colored letters sit on a colored floor without a box around them. It reads the cell
// it draws over, so it is for cells; on a cell that [engine.Canvas.Compose] wrote only the
// background is kept and the pixel above it is lost.
//
// # Animation
//
// An [Animation] is frames and a speed. Give it to a sprite and call [Animation.Advance]
// from the game's Update with the time that passed; the sprite shows the current frame. It
// runs in time, not in calls, so it plays at the same speed at any frame rate. It plays
// once, stopping on its last frame with [Animation.Done] true, or over and over if Loop is
// set. A time that is negative or not a number is ignored.
//
//	walk := sprite.NewAnimation(8, frame0, frame1, frame2, frame3)
//	walk.Loop = true
//	layer.Add(&sprite.Sprite{Animation: walk, Overlay: true})
//
//	func (g *Game) Update(dt float64, in *engine.Input) error {
//		walk.Advance(dt)
//		...
//	}
//
// An animation takes the place of the sprite's Frame, which takes the place of its Cell.
// One with no frame to show draws nothing; it does not fall back.
//
// # Collision
//
// [Sprite.Bounds] is the box a sprite covers, a [geometry.Rect] from its Position, as it is
// and not floored, as wide and tall as what it draws: the animation's current frame, else
// the frame, else a 1 by 1 cell. The whole frame counts, holes included. [Sprite.Overlaps]
// asks whether two boxes share area, and [Layer.Hits] lists the sprites in a layer that
// overlap a box, in draw order, [Layer.All] all of them. Sprites that only touch do not
// collide, and a sprite with nothing to draw, or at a NaN or infinite position, collides
// with nothing.
//
//	for enemy := range enemies.Hits(player.Bounds()) {
//		hurt(enemy)
//	}
//	player.Position = player.Position.Add(player.Bounds().PushOutOf(wall))
//
// Two habits keep a pushed sprite looking right. A push leaves it against the wall to within
// rounding, which can be a hair short of a whole cell, and a position is drawn in the cell it
// is floored to, so round the position after pushing (to a millionth, say) or it can rest a
// cell short. And a push knows how deep a sprite is in a wall, not where it came from, so
// one that moves farther than its own size in a frame can come out the other side: cap the
// time one frame moves it. The layer must not change while Hits or All is running; collect
// what to Remove first.
//
// # Layers
//
// A [Layer] draws its sprites in the order they were added, so later ones cover earlier
// ones. Add, Remove and Clear change what is in it, and Len counts. A sprite at a NaN or
// infinite position, or off screen, is not drawn, and a nil sprite added to a Layer is
// ignored.
package sprite
