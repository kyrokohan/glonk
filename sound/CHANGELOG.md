# Changelog of glonk.dev/sound

The changes to `glonk.dev/sound`, a module of its own with its own versions, tagged `sound/vX.Y.Z`. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the module follows [Semantic Versioning](https://semver.org/). The changes to glonk itself are in the [CHANGELOG](../CHANGELOG.md) at the root.

## [Unreleased]

## [0.1.0] - 2026-11-23

The first release: sounds placed in a scene, mixed, and played on the sound device.

### Added

- A mixer that plays clips where they are in a scene: `Mixer.Play` at a place, `PlayHere` at the listener, and `Loop`, each a `Voice` to move, fade or stop. A sound is heard at its full gain within `Sound.Near` and fades to silence at `Sound.Range`, louder in the nearer ear by the equal-power law, and glides over 20 ms when it or the listener moves, so that nothing clicks (#120).
- `Mixer.Listen(position, right)`, a voice limit where the sound nearest its end gives way, a soft limiter, and `SetVolume`. Every method can be called from any goroutine.
- `LoadWAV` reads 16-bit PCM WAV files of any rate and any number of channels, resampled to 48 kHz mono, and refuses a file over 32 MiB.
- `Mixer.Read`, the mix as stereo samples, for tests with no sound device.
- `Open` plays a mixer on the sound device through [oto](https://github.com/ebitengine/oto), with no C compiler on macOS and Linux. Where there is no device it returns an error, and the game plays on in silence.
- `examples/listen`, three sounds to walk among, and [the Sound guide](https://glonk.dev/manual/sound/).

[Unreleased]: https://github.com/usrgames/glonk/compare/sound/v0.1.0...HEAD
[0.1.0]: https://github.com/usrgames/glonk/releases/tag/sound/v0.1.0
