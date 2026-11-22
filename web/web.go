//go:build js

package web

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

// Run runs the game at fps frames a second in the page until it quits or the page calls glonk.stop,
// and tells the page with glonk.done. Before it starts it adds to glonk what the page calls with what
// happens, each turned into what a terminal would send for it:
//
//	glonk.key(key, code, mods, action)        a KeyboardEvent's key and code, the modifiers (1 Shift,
//	                                          2 Alt, 4 Ctrl, 8 Super) and 1 press, 2 repeat, 3 release
//	glonk.paste(text)                         text pasted into the page
//	glonk.pointer(kind, button, col, row)     "move", "down", "up" or "wheel" in a cell; the button is
//	                                          0 left, 1 middle, 2 right (for a move the one held, or
//	                                          -1), and for the wheel 0 up, 1 down, 2 left, 3 right
//	glonk.focus(in)                           the game gained or lost the keyboard
//	glonk.stop()                              ends the game
//
// A key with Super, Command on a Mac, is the browser's and goes nowhere. The frame is the size the
// page says, with 24-bit color, and is sent whole each time.
//
// Run does not return: the program then waits, idle, for the page to drop it, since a Go program that
// exits with a timer still pending makes wasm_exec.js throw when the timer comes due.
func Run(game engine.Game, fps int) {
	page := js.Global().Get("glonk")
	in, w := io.Pipe()
	// A callback from the page must not block, so what it sends waits in a channel. A full one drops
	// it: nobody does 256 things between two frames.
	sent := make(chan []byte, 256)
	send := func(b []byte) {
		if b == nil {
			return
		}
		select {
		case sent <- b:
		default:
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	funcs := map[string]js.Func{
		"key": js.FuncOf(func(_ js.Value, a []js.Value) any {
			send(keyBytes(a[0].String(), a[1].String(), a[2].Int(), a[3].Int()))
			return nil
		}),
		"paste": js.FuncOf(func(_ js.Value, a []js.Value) any {
			send(pasteBytes(a[0].String()))
			return nil
		}),
		"pointer": js.FuncOf(func(_ js.Value, a []js.Value) any {
			send(pointerBytes(a[0].String(), a[1].Int(), a[2].Int(), a[3].Int()))
			return nil
		}),
		"focus": js.FuncOf(func(_ js.Value, a []js.Value) any {
			send(focusBytes(a[0].Truthy()))
			return nil
		}),
		"stop": js.FuncOf(func(js.Value, []js.Value) any {
			cancel()
			return nil
		}),
	}
	for name, f := range funcs {
		page.Set(name, f)
	}
	go func() {
		for b := range sent {
			w.Write(b)
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

	for name, f := range funcs {
		page.Delete(name)
		f.Release()
	}
	msg := "quit"
	if err != nil && !errors.Is(err, context.Canceled) {
		msg = err.Error()
	}
	page.Call("done", msg)
	select {}
}
