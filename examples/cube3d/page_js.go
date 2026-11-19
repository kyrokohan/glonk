package main

import "glonk.dev/internal/webplay"

// run plays the game in a web page, as glonk.dev does: see internal/webplay. It does not return.
func run(g *cubeGame) error {
	webplay.Run(g, 30)
	return nil
}
