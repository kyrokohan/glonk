# Sound

`glonk.dev/sound` plays sounds at places in a scene. Each is louder as the listener nears it, and in the ear nearer to it. It is a module of its own, so a game without sound gets no audio library:

```
go get glonk.dev/sound
```

## Playing a sound

A `Mixer` mixes the sounds, and a `Speaker` plays the mix on the sound device. This is how `examples/listen` starts:

```go
	mixer := sound.NewMixer(0)
	g := newGame(mixer)
	speaker, err := sound.Open(mixer)
	if err != nil {
		g.trouble = "no sound: " + err.Error()
	} else {
		defer speaker.Close()
	}
```

`Read` gives the mix to a test. Here `tone` is a clip made in code, a stand-in for a WAV file:

```go
	mixer := sound.NewMixer(0)
	mixer.Listen(geometry.Vector3{}, geometry.Vector3{X: 1}) // at the origin, +x to the right

	bell := tone(0.5, time.Second)
	mixer.Play(bell, geometry.Vector3{X: 2, Z: 2}, sound.Sound{Near: 4}) // ahead, to the right

	stereo := make([]float32, 2) // one frame: left, right
	mixer.Read(stereo)
	fmt.Printf("left %.2f, right %.2f\n", stereo[0], stereo[1])
	// Output:
	// left 0.11, right 0.49
```

- **`NewMixer(n)`** plays up to `n` sounds at once, and 0 means 32. A sound past the limit takes the place of the one nearest its end. A loop never gives way.
- **`Play`** puts a sound at a place, **`PlayHere`** at the listener, in both ears, and **`Loop`** plays it until it is stopped.
- **`Open`** once in a program. Where there is no sound device it returns an error, which `listen` shows as it plays on in silence. Every method of the mixer works the same with no speaker.
- **`Read`** is the mix itself, so a test hears a game with no sound device.
- **Every method is safe from any goroutine.** A loud mix is bent toward full scale, never past it, and `SetVolume` sets the volume of all of it.

## Distance

```go
	for _, d := range []float64{1, 4, 7, 10} {
		mixer := sound.NewMixer(0)
		mixer.Play(tone(0.5, time.Second), geometry.Vector3{Z: d}, sound.Sound{Near: 1, Range: 7})
		stereo := make([]float32, 2)
		mixer.Read(stereo)
		fmt.Printf("%2.0f away: %.3f\n", d, stereo[0]+stereo[1])
	}
	// Output:
	//  1 away: 0.707
	//  4 away: 0.177
	//  7 away: 0.000
	// 10 away: 0.000
```

- **Within `Near`** a sound is heard at its full `Gain`. Beyond it the gain falls, as a square, to silence at `Range`.
- **The zero `Sound`** is a gain of 1, a `Near` of 1 and a `Range` of 32, in the units of the scene.
- **In front of the listener** a sound is in both ears, each at 0.707 of it. Turned to one side it is all in that ear.
- **When a sound or the listener moves,** each ear's gain glides to its new level over 20 ms, so nothing clicks.

## Following a camera

```go
	mixer := sound.NewMixer(0)
	position, right := geometry.Vector3{X: 10}, geometry.Vector3{Z: -1} // a camera turned left
	mixer.Listen(position, right)

	mixer.Play(tone(0.5, time.Second), geometry.Vector3{X: 10, Z: -2}, sound.Sound{Near: 4})
	stereo := make([]float32, 2)
	mixer.Read(stereo)
	fmt.Printf("left %.2f, right %.2f\n", stereo[0], math.Round(float64(stereo[1])*100)/100)
	// Output:
	// left 0.00, right 0.50
```

- **`Listen`** takes the listener's place and the way to its right. With a [3D scene](3d.md)'s camera, call `mixer.Listen(cam.Position, cam.Right())` once a frame.
- **`Voice.Move`** moves a sound that is playing, such as one a ship makes as it flies.

## Loops and fades

```go
	mixer := sound.NewMixer(0)
	wind := mixer.Loop(tone(0.5, time.Second), geometry.Vector3{}, sound.Sound{})
	wind.Fade(0.2, 50*time.Millisecond)

	stereo := make([]float32, 2*sound.Rate/10) // a tenth of a second
	mixer.Read(stereo)
	fmt.Printf("after the fade: %.3f\n", stereo[len(stereo)-1])
	// Output:
	// after the fade: 0.071
```

- **`Play`, `PlayHere` and `Loop`** each give a `Voice`. Keep it to change the sound later.
- **`Fade`** takes a sound to a level from 0 to 1 over a time.
- **`Stop`** ends a sound with a fade of 10 ms, so it does not click.
- **A voice that has ended** ignores all three, and so does a nil one, the voice of a sound that was not played.

## WAV files

```go
	clip, err := sound.LoadWAV(&file)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(clip.Duration().Round(time.Millisecond))
	// Output:
	// 100ms
```

- **`LoadWAV` reads any `io.Reader`:** a file, or an embedded asset. Here it is a file made in the test.
- **What it reads:** 16-bit PCM, at any rate and with any number of channels. A clip is mono at 48 kHz, so the channels are mixed down and another rate is converted.
- **What it refuses:** other formats, and files over 32 MiB. The error says why.
- **`NewClip`** makes a clip from samples between -1 and 1, at `sound.Rate`: a sound made in code.

## Where it plays

- **On the machine the game runs on.** Over SSH, that is the server's speakers, not the player's.
- **macOS and Linux need no C compiler.** Linux plays through PulseAudio, or ALSA where there is none.
- **In a browser,** the sound starts once the visitor presses a key or clicks: browsers allow nothing sooner.
- **Windows is untested.** There, a missing device is silence and not an error.

`sound/examples/listen` has three sounds to walk among. `go run glonk.dev/sound/examples/listen@latest` runs it without a clone.
