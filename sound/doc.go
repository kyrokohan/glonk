// Package sound plays sounds where they are in a scene: quieter with distance, louder in the
// nearer ear, and gliding as a sound or the listener moves, so that nothing clicks.
//
// A [Clip] is a sound to play, made in code with [NewClip] or read from a WAV file with
// [LoadWAV]. A [Mixer] plays clips: [Mixer.Play] at a place, [Mixer.PlayHere] at the listener,
// and [Mixer.Loop] over and over, each returning a [Voice] to move, fade or stop. [Mixer.Listen]
// says where the listener is and which way is right, and with a camera of glonk.dev/mesh that
// is one line:
//
//	mixer.Listen(cam.Position, cam.Right())
//
// [Mixer.Read] is the mix, for a test to read with no sound device.
//
// Every method of a Mixer and a Voice can be called from any goroutine, so a game plays sounds
// from its Update while the mix is read from another.
//
// A program that runs over SSH plays its sound on the machine it runs on, not on the one in
// front of the player. In a browser, sound starts only after the player clicks or presses a key.
//
// This is a module of its own, so that a program that does not use sound does not get an audio
// library in its module graph.
package sound
