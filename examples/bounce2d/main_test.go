package main

import (
	"errors"
	"math"
	"strings"
	"testing"

	"glonk.dev/engine"
)

func newFrame(t *testing.T, w, h int) *engine.FrameBuffer {
	t.Helper()
	fb, err := engine.NewFrameBuffer(w, h)
	if err != nil {
		t.Fatalf("unable to initialize frame buffer: %v", err)
	}
	return fb
}

// newSizedGame returns a game that has been told its frame is w x h, as Run does
// before the first Update.
func newSizedGame(w, h int) *bounceGame {
	g := newBounceGame()
	g.Resize(w, h)
	return g
}

func TestBallMovesByVelocityTimesTime(t *testing.T) {
	g := newSizedGame(64, 24)
	if err := g.Update(0.1, &engine.Input{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.ball.Position.X != 13 || g.ball.Position.Y != 11.5 {
		t.Errorf("wanted the ball at (13, 11.5), got (%v, %v)", g.ball.Position.X, g.ball.Position.Y)
	}
}

func TestBallBouncesOffEachWall(t *testing.T) {
	// In a 64x24 frame the ball lives in columns 1 to 62 and rows 1 to 22.
	tests := []struct {
		name           string
		x, y           float64
		vx, vy         float64
		wantX, wantY   float64
		wantVxPositive bool
		wantVyPositive bool
	}{
		{"right wall", 62.5, 10, 30, 15, 62, 10, false, true},
		{"left wall", 0.5, 10, -30, 15, 1, 10, true, true},
		{"bottom wall", 10, 22.5, 30, 15, 10, 22, true, false},
		{"top wall", 10, 0.5, 30, -15, 10, 1, true, true},
		{"inside moves freely", 10, 10, 30, 15, 10, 10, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := newSizedGame(64, 24)
			g.ball.Position.X, g.ball.Position.Y, g.velocity.X, g.velocity.Y = tt.x, tt.y, tt.vx, tt.vy

			if err := g.Update(0, &engine.Input{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if g.ball.Position.X != tt.wantX || g.ball.Position.Y != tt.wantY {
				t.Errorf("wanted the ball at (%v, %v), got (%v, %v)", tt.wantX, tt.wantY, g.ball.Position.X, g.ball.Position.Y)
			}
			if (g.velocity.X > 0) != tt.wantVxPositive || (g.velocity.Y > 0) != tt.wantVyPositive {
				t.Errorf("wanted velocity signs (%v, %v), got (%v, %v)", tt.wantVxPositive, tt.wantVyPositive, g.velocity.X, g.velocity.Y)
			}
		})
	}
}

// The box is the size of the window: the ball bounces off the walls of the frame
// it was last told about, not off a fixed size.
func TestBallBouncesInsideTheFrameLastResizedTo(t *testing.T) {
	g := newBounceGame()

	g.Resize(100, 30)
	g.ball.Position.X, g.ball.Position.Y, g.velocity.X = 90, 10, 30
	if err := g.Update(0.01, &engine.Input{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.velocity.X <= 0 {
		t.Errorf("wanted the ball to fly on past 64 columns in a 100 column frame, vx=%v", g.velocity.X)
	}

	g.Resize(40, 12)
	if err := g.Update(0, &engine.Input{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.ball.Position.X > 38 || g.ball.Position.Y > 10 || g.ball.Position.X < 1 || g.ball.Position.Y < 1 {
		t.Errorf("wanted the ball back inside a 40x12 frame after it shrank, got (%v, %v)", g.ball.Position.X, g.ball.Position.Y)
	}
}

func TestDrawShowsTheBoxTheBallAndTheKey(t *testing.T) {
	g := newSizedGame(40, 12)
	fb := newFrame(t, 40, 12)
	g.Draw(fb)

	if fb.At(0, 0).Character != '┌' || fb.At(39, 0).Character != '┐' || fb.At(0, 11).Character != '└' || fb.At(39, 11).Character != '┘' {
		t.Errorf("wanted a box around the whole frame, corners were %q %q %q %q",
			fb.At(0, 0).Character, fb.At(39, 0).Character, fb.At(0, 11).Character, fb.At(39, 11).Character)
	}
	if got := strings.Split(fb.String(), "\n")[11]; !strings.Contains(got, " Esc quits ") {
		t.Errorf("wanted the key hint on the bottom border, got %q", got)
	}
	if got := fb.At(int(g.ball.Position.X), int(g.ball.Position.Y)).Character; got != 'o' {
		t.Errorf("wanted the ball drawn, got %q", got)
	}
}

func TestDrawInTinyFramesKeepsTheCornersAndDoesNotPanic(t *testing.T) {
	for _, size := range [][2]int{{1, 1}, {2, 2}, {3, 3}, {5, 5}, {10, 4}, {14, 4}, {15, 4}} {
		fb := newFrame(t, size[0], size[1])
		g := newSizedGame(size[0], size[1])
		g.Draw(fb)
		if err := g.Update(0.1, &engine.Input{}); err != nil {
			t.Fatalf("%v: unexpected error: %v", size, err)
		}

		if size[0] >= 2 && size[1] >= 2 && fb.At(size[0]-1, size[1]-1).Character != '┘' {
			t.Errorf("%v: wanted the bottom right corner kept, got %q", size, fb.At(size[0]-1, size[1]-1).Character)
		}
	}
}

func TestEscapeQuitsAndOtherKeysDoNot(t *testing.T) {
	g := newSizedGame(64, 24)
	kb := engine.NewKeyboard()

	kb.Press('w', 'q', engine.KeyEnter, engine.KeyUp)
	x, y := g.ball.Position.X, g.ball.Position.Y
	if err := g.Update(0.1, kb.Input()); err != nil {
		t.Fatalf("wanted other keys to be ignored, got %v", err)
	}
	if math.Abs(g.ball.Position.X-(x+3)) > 1e-9 || math.Abs(g.ball.Position.Y-(y+1.5)) > 1e-9 {
		t.Errorf("wanted the ball to fly on as usual, got (%v, %v) from (%v, %v)", g.ball.Position.X, g.ball.Position.Y, x, y)
	}
	kb.EndFrame()

	kb.Press(engine.KeyEscape)
	if err := g.Update(0.1, kb.Input()); !errors.Is(err, engine.ErrQuit) {
		t.Errorf("wanted ErrQuit on Escape, got %v", err)
	}
}
