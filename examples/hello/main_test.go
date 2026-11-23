package main

import (
	"errors"
	"testing"

	"glonk.dev/engine"
)

func TestEscapeQuits(t *testing.T) {
	kb := engine.NewKeyboard()
	kb.Press(engine.KeyEscape)

	err := (&game{}).Update(1.0/30, kb.Input())

	if !errors.Is(err, engine.ErrQuit) {
		t.Errorf("wanted ErrQuit, got %v", err)
	}
}

func TestTheAtWalksAcrossTheBox(t *testing.T) {
	g := &game{}
	g.Update(0.25, engine.NewKeyboard().Input()) // a quarter of a second at 20 cells a second

	fb, _ := engine.NewFrameBuffer(12, 3)
	g.Draw(fb)

	want := "┌──────────┐\n" +
		"│    @     │\n" +
		"└──────────┘"
	if got := fb.String(); got != want {
		t.Errorf("wanted\n%s\ngot\n%s", want, got)
	}
}
