# Terminals, colors and limits

glonk is built to work in most terminals. This guide says what it needs from one, what it uses where a terminal has more, and what it does not do.

## What a terminal needs

- **The basics need only three things:** the alternate screen, cursor movement, and the 256-color palette.
- **Box lines and the pixel canvas need a Unicode font,** for `┌─┐│└┘` and `▀▄`. Nearly every monospace font has them. Character shading needs only ASCII.
- **Cells are taken as twice as tall as they are wide.** That is what makes a cube look like a cube. Set `Camera.CellAspect` for a font that is not.
- **Line wrapping is turned off while a game runs,** and all of it is undone when `Run` returns.
- **glonk is developed on macOS,** and tested mainly in Ghostty.
- **Windows is untested.**

## What it uses where a terminal has it

- **True color:** exact RGB colors, in place of the nearest of the palette.
- **The kitty keyboard protocol:** key releases, so several keys can be held. See [Keys, text and the mouse](input.md#terminals-without-key-releases).
- **Bracketed paste:** a paste as one string, not as key presses.
- **Focus reporting:** everything drawn again when the window comes back to the front.
- **The decimal mouse mode, 1006:** the mouse, when `Config.Mouse` asks for it.
- **The pointer in pixels, mode 1016:** the pointer finer than a cell.
- **Synchronized updates:** each frame shown all at once. A terminal without them ignores the request.

Which terminals were tried:

- **Ghostty and Apple's Terminal** both send the decimal mouse mode.
- **Ghostty** reports the pointer in pixels. Apple's Terminal does not, and the pointer moves a cell at a time there.
- `go run glonk.dev/examples/input@latest` shows what your terminal sends.

## Colors

- **An RGB color is sent exactly** when `COLORTERM` says the terminal has true color: `truecolor` or `24bit`. Terminals that have it set that themselves.
- **Otherwise it is sent as the nearest of the palette colors 16 to 255.**
- **Inside tmux or over ssh, `COLORTERM` often does not arrive.** Export `COLORTERM=truecolor`, or set `Config.Colors` to `engine.ColorsTrue`.
- **Forcing true color on a terminal that cannot show it** gives wrong colors. `engine.Colors256` turns it off.
- **A palette color is always sent as the palette color,** so colors 0 to 15 follow the player's theme.
- **Apple's Terminal does not set `COLORTERM`,** and keeps the 256-color look.
- **`engine.Probe`** says before a game starts whether there is a terminal at all, whether it has true color and reports key releases, and its size.

## Apple's Terminal and half blocks

Apple's Terminal draws a faint line along the edges of half blocks, since its block characters fall a little short of the cell. glonk detects it, and `Compose` draws solid cells there instead: no line, and a picture a pixel blockier. `Canvas.Seamless` overrides the choice, for one inside tmux, which hides the variable glonk reads.

## The size of the window

- **A terminal that reports no size is taken as 80 by 24.** That is a pseudo-terminal nobody gave a size, as under `script` in a CI step, or in some container consoles.
- **`Run` uses 80 columns or 24 rows for each axis left at 0,** until the terminal reports a size, and follows it from then on.
- **Resizing can flicker for one frame.**

## What glonk sends

Each frame sends only the cells that changed since the frame before, as one update. A screen that does not change sends nothing. Over ssh, the bytes a frame takes decide whether a game is playable: [Frame numbers](first-game.md#frame-numbers) says how to see them.

## Something wrong

- **A bug in how glonk leaves the terminal** after a crash, a signal or a panic it says it handles is a security problem: see [SECURITY.md](../SECURITY.md).
- **The endings it cannot clean up after** are in [How a run ends](first-game.md#how-a-run-ends).
- **Anything else** is an [issue](https://github.com/usrgames/glonk/issues).
