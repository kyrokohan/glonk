// Package web runs a glonk game in a web page: the engine without a terminal, the page's keys, pastes
// and pointer in, and its frames out, drawn by xterm.js. It is built for the browser only, with
// GOOS=js GOARCH=wasm; elsewhere it is empty.
//
// A game keeps its terminal entry in a file built for everything but the browser, and adds one for
// the page:
//
//	// terminal.go
//	//go:build !js
//
//	func run(g *game) error {
//		return engine.Run(context.Background(), g, engine.Config{FPS: 30})
//	}
//
//	// page_js.go
//	func run(g *game) error {
//		web.Run(g, 30)
//		return nil
//	}
//
// Build it with GOOS=js GOARCH=wasm go build, and put beside it, on the page, wasm_exec.js from the
// same Go ($(go env GOROOT)/lib/wasm), xterm.js and its WebGL addon, and glonk.js from this package's
// folder, which starts the game in an element and wires the keys, a paste, the pointer and the wheel
// to it. The README's "A game in a web page" shows a whole page.
//
// The page and Run meet in one object in the global scope, glonk, which glonk.js sets up: the page
// gives it cols and rows, the size of the frame in cells, frame(bytes, n), called with each write of a
// frame, of which the first n bytes are new, and done(message), called once when the game ends with
// "quit" or what went wrong. Run adds key, paste, pointer, focus and stop for the page to call.
package web
