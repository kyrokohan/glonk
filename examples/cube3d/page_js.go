package main

import "glonk.dev/web"

// run plays the game in a web page, as glonk.dev's cover does: see glonk.dev/web. It does not return.
func run(g *cubeGame) error {
	web.Run(g, 30)
	return nil
}
