//go:build js

// Package webplay runs a game in a web page, for glonk.dev: the engine headless, the page's keys
// in and its frames out, through one object the page puts in the global scope:
//
//	glonk.cols, glonk.rows  the size of the frame, in cells
//	glonk.frame(bytes, n)   called with each write of a frame: the first n bytes are new
//	glonk.done(message)     called once when the game ends: "quit", or what went wrong
//
// and two functions Run adds to it for the page to call:
//
//	glonk.keys(bytes)       key bytes, as the kitty keyboard protocol sends them
//	glonk.stop()            ends the game
package webplay

import (
	"context"
	"errors"
	"io"
	"syscall/js"

	"glonk.dev/engine"
)

// pageOut hands every write of a frame to the page.
type pageOut struct {
	frame js.Value
	buf   js.Value
	size  int
}

func (o *pageOut) Write(p []byte) (int, error) {
	if len(p) > o.size {
		o.size = len(p)
		o.buf = js.Global().Get("Uint8Array").New(o.size)
	}
	js.CopyBytesToJS(o.buf, p)
	o.frame.Invoke(o.buf, len(p))
	return len(p), nil
}

// Run runs the game at fps frames a second until it quits or the page calls glonk.stop, and
// tells the page with glonk.done. It does not return: the program then waits, idle, for the page
// to drop it, since a Go program that exits with a timer still pending makes wasm_exec.js throw
// when the timer comes due.
func Run(game engine.Game, fps int) {
	page := js.Global().Get("glonk")
	in, keys := io.Pipe()
	// A callback from the page must not block, so key bytes wait in a channel. A full one drops
	// them: nobody types 256 keys between two frames.
	typed := make(chan []byte, 256)
	onKeys := js.FuncOf(func(_ js.Value, args []js.Value) any {
		b := make([]byte, args[0].Get("length").Int())
		js.CopyBytesToGo(b, args[0])
		select {
		case typed <- b:
		default:
		}
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	onStop := js.FuncOf(func(js.Value, []js.Value) any {
		cancel()
		return nil
	})
	page.Set("keys", onKeys)
	page.Set("stop", onStop)
	go func() {
		for b := range typed {
			keys.Write(b)
		}
	}()

	err := engine.Run(ctx, game, engine.Config{
		Width:  page.Get("cols").Int(),
		Height: page.Get("rows").Int(),
		FPS:    fps,
		Colors: engine.ColorsTrue,
		Input:  in,
		Output: &pageOut{frame: page.Get("frame")},
	})

	page.Delete("keys")
	page.Delete("stop")
	onKeys.Release()
	onStop.Release()
	msg := "quit"
	if err != nil && !errors.Is(err, context.Canceled) {
		msg = err.Error()
	}
	page.Call("done", msg)
	select {}
}
