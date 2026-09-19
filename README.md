# Stardate Type Drop

A Star Trek-themed typing game built with [Ebitengine](https://ebitengine.org/) in Go. Escape pods launch from a ship and fall toward a gravity well — type fast to rescue them.

## Game Modes

### Random Mode
Words fall from a ship at the top of the screen toward a planet near the bottom. Type each word before it reaches the planet. Game over when any word passes the bottom.

### Passage Mode
Star Trek passages scroll on screen like a teleprompter. 5 lines visible at once, auto-scrolling to center the current line. Mistypes skip the current word (shown in red with strikethrough) rather than penalizing. 1.5-second crossfade transition between passages.

## How Words Fall (Random Mode)

- Spawning is beat-synced: 100 BPM = one beat every 0.6 seconds
- Words only spawn on beats, gated by a cooldown
- Base spawn gap starts at ~1.2 seconds and shrinks by ~0.03s per level, minimum ~0.2s
- Adaptive targeting: the game tries to keep ~6 words on screen at once. If there are more than 8, spawning slows. If fewer than 4, it speeds up
- Words launch from the ship at downward angles with slight horizontal drift — they don't fall straight down
- As words approach the planet, they curve toward it and accelerate

## How It Gets Harder

| Factor | Level 1 | Scaling | Cap |
| --- | --- | --- | --- |
| Fall speed | 0.3 px/frame | +0.03 per level | 2.5 px/frame |
| Spawn gap | ~1.2s | Shrinks ~0.03s per level | ~0.2s |
| Level up | Every 5 words completed | — | — |

The game also adapts to word density — too many words on screen slows spawning, too few speeds it up. Early game is gentle; by level 15 you're at max speed with tight spawns.

## Scoring

| Word Length | Points |
| --- | --- |
| 3-4 letters | 1 |
| 5-6 letters | 3 |
| 7+ letters | 5 |

## Power-ups and Bombs

- **Slow Motion**: Earn a charge every 10 words. Press Space to activate — 10 seconds of paused spawning and visual slowdown. Swaps to a different bass track.
- **Bombs**: After a word is completed, it can arm as a bomb (2-second timer). If not defused, it explodes within 100px radius, destroying nearby words but not counting toward your score.

## Audio

- Bass and percussion volumes scale with level
- Slow-motion swaps to a different bass track
- Sound effects on every hit, miss, level-up, and power-up activation

## Controls

| Key | Action |
| --- | --- |
| Letters | Type to match falling words (auto-targets lowest matching word) |
| Tab | Toggle auto-fire mode |
| Space | Activate slow-motion power-up |
| Escape | Quit |
| 1 / 2 | Switch between Random and Passage mode (on game-over screen) |

## Building

```bash
python scripts/build.py --verify
```

Or on Unix:

```bash
make build
```

Artifacts are written to `releases/{goos}/{goarch}/app/latest[.exe]`.

## Running

```bash
python scripts/run_artifact.py
```

## Project Layout

```text
cmd/app/main.go              Game source (Ebitengine, ~1900 lines)
cmd/app/words.go             Word list for Random mode
cmd/app/_assets/             Fonts, sound effects
scripts/build.py             Build pipeline
scripts/run_artifact.py      Artifact runner
releases/                    Build output (ignored by git)
agentic-pipelines/           Optional Ollama/governance scaffolding
```

## Credits

Built with Ebitengine v2.9.9. Fonts: Antonio (Bold, Light, Regular). Star Trek passages are paraphrases of TNG-era scenarios.
