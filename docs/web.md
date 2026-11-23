# A game in a web page

The `web` package runs a game in a web page: the same game as in the terminal, so that its own page can have a Play button. The cover of [glonk.dev](https://glonk.dev) plays `examples/cube3d` this way.

## Two entry files

A game has one way in for the terminal and one for the page. Keep the terminal's in a file built for everything but the browser:

```go
//go:build !js

package main

import (
	"context"

	"glonk.dev/engine"
)

// run plays the game in the terminal.
func run(g *cubeGame) error {
	return engine.Run(context.Background(), g, engine.Config{FPS: 30})
}
```

Add one for the page. A file whose name ends in `_js.go` is built for the browser only:

```go
package main

import "glonk.dev/web"

// run plays the game in a web page, as glonk.dev's cover does: see glonk.dev/web. It does not return.
func run(g *cubeGame) error {
	web.Run(g, 30)
	return nil
}
```

- These are `terminal.go` and `page_js.go` of [examples/cube3d](../examples/cube3d).
- `main` calls `run` and does not ask which of the two it got.
- `web.Run(game, fps)` does not return.

## Build it

```sh
GOOS=js GOARCH=wasm go build -trimpath -ldflags=-s -o game.wasm . && gzip -9n game.wasm
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" .
cp "$(go list -m -f '{{.Dir}}' glonk.dev)/web/glonk.js" .
```

- **`game.wasm.gz`** is the game.
- **`wasm_exec.js`** starts a Go program in a browser. It must come from the same Go that built the game: one from another version cannot start it.
- **`glonk.js`** is the page's half of glonk. Copy it from the version of glonk the game was built with.

## What draws it

The page draws the game with [xterm.js](https://xtermjs.org), a terminal for the browser. It needs three of its files:

| File | From the npm package |
|---|---|
| `xterm.js` | `@xterm/xterm`, in `lib` |
| `xterm.css` | `@xterm/xterm`, in `css` |
| `addon-webgl.js` | `@xterm/addon-webgl`, in `lib` |

- glonk.dev uses `@xterm/xterm` 6.0.0 and `@xterm/addon-webgl` 0.19.0.
- Load the WebGL addon. Without it xterm.js still draws the game, with faint seams through the pixel canvas.

## The page

The page loads the stylesheet and four scripts, in this order, and starts the game when the player asks, here from a button:

```html
<link rel="stylesheet" href="xterm.css">
<script src="xterm.js"></script>
<script src="addon-webgl.js"></script>
<script src="wasm_exec.js"></script>
<script src="glonk.js"></script>
<div id="game"></div>
<button id="play">Play</button>
<script>
  document.getElementById("play").onclick = async () => {
    const game = await glonkPlay(document.getElementById("game"), { game: "game.wasm.gz", cols: 80, rows: 24 });
    console.log("the game ended:", await game.done);
  };
</script>
```

- **`glonkPlay(box, {game, cols, rows, terminal})`** starts the game in the element `box`, in `cols` by `rows` cells.
- **`game`** is the address of the build. A name that ends in `.gz` is unpacked in the page.
- **`terminal`** is passed on to xterm.js, for the font and the colors.
- **It gives back `done` and `stop`.** `done` is a promise of how the game ended: `"quit"`, or what went wrong. `stop()` ends the game.
- **One game runs on a page at a time.** A second `glonkPlay` while one runs is an error.

## What the player gets

- **The keyboard is the game's while it has the focus,** except Tab and Command. Tab leaves the game, and Command is the browser's.
- **Keys arrive with their releases,** so several can be held at once.
- **A paste arrives as a paste,** and the pointer by the cell it is over.
- **The wheel and the right button are the game's only while it has the keyboard,** so a visitor can still scroll the page past it.
- **A hidden tab draws nothing.** The browser slows a hidden tab's timers to about once a second, so the game runs on slowly. The first frame after the tab shows again repaints all of it.

## No terminal in a browser

glonk compiles for `GOOS=js GOARCH=wasm`, and CI checks that it still does. A browser has no terminal, so `engine.Run` there needs `Config.Input`, `Output`, `Width` and `Height`. `web.Run` gives it all four: the page's keys go in, and each frame goes out to the page, at the size the page asked for.

The [package's documentation](https://pkg.go.dev/glonk.dev/web?GOOS=js&GOARCH=wasm) says what the page and `Run` call on each other, for a page that does without `glonk.js`.

## Not included

Phones and touch, and a change of size while the game runs. Sound is a module of its own: see [Sound](sound.md).
