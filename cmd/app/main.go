package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"math/rand/v2"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

// =============================================================================
// Embedded Assets
// =============================================================================

//go:embed _assets/Antonio-Bold.ttf
var antonioBold []byte

//go:embed _assets/Antonio-Light.ttf
var antonioLight []byte

//go:embed _assets/Antonio-Regular.ttf
var antonioRegular []byte

//go:embed _assets/bass1.wav
var bass1Data []byte

//go:embed _assets/bass2.wav
var bass2Data []byte

//go:embed _assets/synth.wav
var synthData []byte

//go:embed _assets/perc.wav
var percData []byte

//go:embed _assets/hit.wav
var hitData []byte

//go:embed _assets/miss.wav
var missData []byte

//go:embed _assets/click.wav
var clickData []byte

//go:embed _assets/levelup.wav
var levelupData []byte
//go:embed _assets/powerup.wav
var powerupData []byte
//go:embed _assets/menu.wav
var menuData []byte

// =============================================================================
// Constants
// =============================================================================

const (
	ScreenWidth  = 720
	ScreenHeight = 720
	FrameRate    = 60
	BPM          = 100
	BeatFrames   = FrameRate * 60 / BPM  // 36 frames per beat at 100 BPM
	BaseSpeed    = 0.3
	SpeedStep    = 0.03
	MaxSpeed     = 2.5
	StreakUp     = 8   // speed up after N correct in a row
	StreakDown   = 3   // slow down after N misses in a row
	MaxWordMisses = 3    // delete word after N misses
	BombArmTime  = 120   // frames until armed bomb explodes
	BombRadius   = 100.0 // explosion radius in pixels
	PlanetX      = float64(ScreenWidth) / 2  // planet center X
	PlanetY      = float64(ScreenHeight) * 0.85  // planet near bottom
	PlanetRadius = 40.0
	Gravity      = 0.02  // gravitational pull
	MinSpawnGap  = 12
	MaxSpawnGap  = 70
	WordY        = 80   // Y position for target word display
	DangerLine   = ScreenHeight - 100
	SlowDuration = 600  // 10 seconds at 60fps

	// Passage mode constants
	PassageLineHeight  = 38  // pixels between line baselines
	PassageMargin      = 60  // horizontal margin each side
	PassageVisibleLines = 5  // lines visible at once
	PassageScrollBase  = 0.5 // base scroll speed (pixels/frame)
	PassageFadeLines   = 2   // lines of fade at top/bottom
)

// =============================================================================
// Color Definitions
// =============================================================================

var (
	ColorPurple    = color.RGBA{180, 80, 255, 255}
	ColorOrange    = color.RGBA{255, 140, 40, 255}
	ColorGold      = color.RGBA{255, 200, 80, 255}
	ColorLavender  = color.RGBA{140, 100, 255, 255}
	ColorRedOrange = color.RGBA{255, 100, 60, 255}
	ColorWhite     = color.RGBA{240, 240, 255, 255}
	ColorDim       = color.RGBA{120, 100, 150, 255}
	ColorBg        = color.RGBA{20, 10, 30, 255}
	levelHue       float64
	ColorTyped     = color.RGBA{255, 140, 40, 255}
	ColorNext      = color.RGBA{255, 200, 80, 255}
	ColorRescue    = color.RGBA{255, 80, 200, 255}
	ColorDanger    = color.RGBA{255, 60, 60, 255}
)

// =============================================================================
// Font System
// =============================================================================

var (
	gameFont    font.Face // Light - falling words (18pt)
	targetFont  font.Face // Bold - target word display (36pt)
	hudFont     font.Face // Regular - HUD elements (18pt)
	passageFont font.Face // Regular - passage mode text (24pt)
)

// loadFont initializes all font faces from embedded TTF data.
func loadFont() {
	if antonioBold != nil {
		if f, err := opentype.Parse(antonioBold); err == nil {
			face, _ := opentype.NewFace(f, &opentype.FaceOptions{
				Size: 36, DPI: 72, Hinting: font.HintingFull,
			})
			targetFont = face
		}
	}

	if antonioLight != nil {
		if f, err := opentype.Parse(antonioLight); err == nil {
			face, _ := opentype.NewFace(f, &opentype.FaceOptions{
				Size: 18, DPI: 72, Hinting: font.HintingFull,
			})
			gameFont = face
		}
	}

	if antonioRegular != nil {
		if f, err := opentype.Parse(antonioRegular); err == nil {
			face, _ := opentype.NewFace(f, &opentype.FaceOptions{
				Size: 18, DPI: 72, Hinting: font.HintingFull,
			})
			hudFont = face
		}
		if f, err := opentype.Parse(antonioRegular); err == nil {
			face, _ := opentype.NewFace(f, &opentype.FaceOptions{
				Size: 24, DPI: 72, Hinting: font.HintingFull,
			})
			passageFont = face
		}
	}
}

// =============================================================================
// Audio System
// =============================================================================

// Loop wraps an io.ReadSeeker to loop playback.
type Loop struct {
	r io.ReadSeeker
}

func (l *Loop) Read(p []byte) (int, error) {
	n, err := l.r.Read(p)
	if err == io.EOF {
		l.r.Seek(0, io.SeekStart)
		n2, err2 := l.r.Read(p[n:])
		return n + n2, err2
	}
	return n, err
}

func (l *Loop) Seek(offset int64, whence int) (int64, error) {
	return l.r.Seek(offset, whence)
}

// Audio manages all game audio: music loops and sound effects.
type Audio struct {
	ctx         *audio.Context
	bass        *audio.Player
	synth       *audio.Player
	perc        *audio.Player
	slowBass    *audio.Player
	hitData     []byte
	missData    []byte
	clickData   []byte
	powerData   []byte
	levelupData []byte
	menuData    []byte
}

// loadLoopData creates a looping audio player from WAV data.
func loadLoopData(ctx *audio.Context, data []byte) *audio.Player {
	if data == nil {
		return nil
	}
	stream, err := wav.DecodeWithoutResampling(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	p, err := audio.NewPlayer(ctx, &Loop{r: stream})
	if err != nil {
		return nil
	}
	return p
}

// NewAudio creates a new Audio system with all sound effects and music loops.
func NewAudio() *Audio {
	ctx := audio.NewContext(44100)
	if ctx == nil {
		return nil
	}
	return &Audio{
		ctx:       ctx,
		bass:      loadLoopData(ctx, bass2Data),
		synth:     loadLoopData(ctx, synthData),
		perc:      loadLoopData(ctx, percData),
		slowBass:  loadLoopData(ctx, bass1Data),
		hitData:   hitData,
		missData:  missData,
		clickData: clickData,
		powerData: powerupData,
	}
}

// play plays a one-shot sound effect at 0.8 volume.
func (a *Audio) play(data []byte) {
	if a.ctx == nil || data == nil {
		return
	}
	stream, err := wav.DecodeWithoutResampling(bytes.NewReader(data))
	if err != nil {
		return
	}
	p, err := audio.NewPlayer(a.ctx, stream)
	if err != nil {
		return
	}
	p.SetVolume(0.8)
	p.Play()
}

// Sound effect helpers
func (a *Audio) playClick()   { a.play(a.clickData) }
func (a *Audio) playHit()     { a.play(a.hitData) }
func (a *Audio) playMiss()    { a.play(a.missData) }
func (a *Audio) playPowerup() { a.play(a.powerData) }
func (a *Audio) playLevelup() { a.play(a.levelupData) }
func (a *Audio) playMenuMusic() { a.play(a.menuData) }

// startMusic starts all music loops (initially at low volume).
func (a *Audio) startMusic() {
	if a.bass != nil {
		a.bass.SetVolume(0)
		a.bass.Play()
	}
	if a.slowBass != nil {
		a.slowBass.SetVolume(0)
		a.slowBass.Play()
	}
	if a.synth != nil {
		a.synth.SetVolume(0.4)
		a.synth.Play()
	}
	if a.perc != nil {
		a.perc.SetVolume(0)
		a.perc.Play()
	}
}

// Music volume control
func (a *Audio) setBassVolume(v float64)     { if a.bass != nil { a.bass.SetVolume(v) } }
func (a *Audio) setSlowBassVolume(v float64) { if a.slowBass != nil { a.slowBass.SetVolume(v) } }
func (a *Audio) setPercVolume(v float64)     { if a.perc != nil { a.perc.SetVolume(v) } }

// stopMusic pauses all music loops.
func (a *Audio) stopMusic() {
	if a.bass != nil { a.bass.Pause() }
	if a.slowBass != nil { a.slowBass.Pause() }
	if a.synth != nil { a.synth.Pause() }
	if a.perc != nil { a.perc.Pause() }
}

// =============================================================================
// Particle System
// =============================================================================

// Particle represents a single visual particle.
type Particle struct {
	x, y    float64
	vx, vy  float64
	life    float64
	decay   float64
	size    float64
	color   color.RGBA
	image   *ebiten.Image
}

// ParticleSystem manages visual particle effects for word completion.
type ParticleSystem struct {
	particles []*Particle
	circle    *ebiten.Image // Small circle image for rendering
}

// NewParticleSystem creates a new particle system with a circle sprite.
func NewParticleSystem() *ParticleSystem {
	r := 4
	size := r * 2
	circleImg := image.NewRGBA(image.Rect(0, 0, size, size))
	cx, cy := float64(r), float64(r)
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			dx := float64(px) - cx
			dy := float64(py) - cy
			if math.Sqrt(dx*dx+dy*dy) <= float64(r) {
				circleImg.Set(px, py, color.RGBA{255, 255, 255, 255})
			}
		}
	}
	return &ParticleSystem{
		circle: ebiten.NewImageFromImage(circleImg),
	}
}

// Spawn adds a single particle at the given position with the specified color.
func (ps *ParticleSystem) Spawn(x, y float64, c color.RGBA, count int) {
	for i := 0; i < count; i++ {
		angle := rand.Float64() * math.Pi * 2
		speed := 1.0 + rand.Float64()*4.0
		ps.particles = append(ps.particles, &Particle{
			x:     x, y: y,
			vx:    math.Cos(angle) * speed,
			vy:    math.Sin(angle)*speed - 1.0,
			life:  1.0,
			decay: 0.015 + rand.Float64()*0.025,
			size:  1.0 + rand.Float64()*2.0,
			color: c,
		})
	}
}

// SpawnBurst creates a burst of colorful particles when a word is completed.
func (ps *ParticleSystem) SpawnBurst(x, y float64, words string) {
	palette := []color.RGBA{ColorPurple, ColorOrange, ColorGold, ColorLavender, ColorRedOrange}
	count := 8 + len(words)*2
	for i := 0; i < count; i++ {
		ps.Spawn(x, y, palette[rand.IntN(len(palette))], 1)
	}
}

// SpawnRescueBurst creates a rescue-themed particle burst.
func (ps *ParticleSystem) SpawnRescueBurst(x, y float64) {
	for i := 0; i < 20; i++ {
		ps.Spawn(x, y, ColorRescue, 1)
	}
}

// Update advances all particles and removes dead ones.
func (ps *ParticleSystem) Update() {
	alive := ps.particles[:0]
	for _, p := range ps.particles {
		p.x += p.vx
		p.y += p.vy
		p.vy += 0.05
		p.vx *= 0.98
		p.life -= p.decay
		if p.life > 0 {
			alive = append(alive, p)
		}
	}
	ps.particles = alive
}

// Draw renders all active particles to the screen.
func (ps *ParticleSystem) Draw(screen *ebiten.Image) {
	for _, p := range ps.particles {
		alpha := uint8(p.life * 255)
		if alpha == 0 {
			continue
		}
		op := &ebiten.DrawImageOptions{}
		s := p.size * p.life
		op.GeoM.Scale(s, s)
		op.GeoM.Translate(p.x-s/2, p.y-s/2)
		op.ColorScale.ScaleWithColor(color.RGBA{p.color.R, p.color.G, p.color.B, alpha})
		screen.DrawImage(ps.circle, op)
	}
}

// =============================================================================
// Starfield
// =============================================================================

// Star represents a single star in the background.
type Star struct {
	x, y       float64
	speed      float64
	brightness uint8
}

// Starfield manages the scrolling star background.
type Starfield struct {
	stars []Star
	dot   *ebiten.Image
}

// NewStarfield creates a new starfield with 80 randomly placed stars.
func NewStarfield() *Starfield {
	s := &Starfield{}
	// Create a small dot for stars
	r := 2
	size := r * 2
	dotImg := image.NewRGBA(image.Rect(0, 0, size, size))
	cx, cy := float64(r), float64(r)
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			dx := float64(px) - cx
			dy := float64(py) - cy
			if math.Sqrt(dx*dx+dy*dy) <= float64(r) {
				dotImg.Set(px, py, color.RGBA{255, 255, 255, 255})
			}
		}
	}
	s.dot = ebiten.NewImageFromImage(dotImg)
	for i := 0; i < 80; i++ {
		s.stars = append(s.stars, Star{
			x:          rand.Float64() * float64(ScreenWidth),
			y:          rand.Float64() * float64(ScreenHeight),
			speed:      0.1 + rand.Float64()*0.4,
			brightness: uint8(60 + rand.IntN(140)),
		})
	}
	return s
}

// Update scrolls stars downward and wraps them.
func (s *Starfield) Update() {
	for i := range s.stars {
		s.stars[i].y += s.stars[i].speed
		if s.stars[i].y > float64(ScreenHeight) {
			s.stars[i].y = 0
			s.stars[i].x = rand.Float64() * float64(ScreenWidth)
		}
	}
}

// Draw renders all stars to the screen.
func (s *Starfield) Draw(screen *ebiten.Image) {
	for _, star := range s.stars {
		c := color.RGBA{star.brightness, star.brightness, star.brightness + 20, 255}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(0.5, 0.5)
		op.GeoM.Translate(star.x, star.y)
		op.ColorScale.ScaleWithColor(c)
		screen.DrawImage(s.dot, op)
	}
}

// =============================================================================
// Hue Rotation
// =============================================================================

// hueRotate shifts a color's hue by the given degrees while preserving alpha.
func hueRotate(c color.RGBA, deg float64) color.RGBA {
	if deg == 0 {
		return c
	}
	r := float64(c.R) / 255
	g := float64(c.G) / 255
	b := float64(c.B) / 255

	// RGB to HSV
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	d := max - min
	h := 0.0
	if d > 0 {
		if max == r {
			h = math.Mod((g-b)/d, 6)
		} else if max == g {
			h = (b-r)/d + 2
		} else {
			h = (r-g)/d + 4
		}
		h *= 60
		if h < 0 {
			h += 360
		}
	}
	s := 0.0
	if max > 0 {
		s = d / max
	}
	v := max

	// Rotate hue
	h = math.Mod(h+deg, 360)
	if h < 0 {
		h += 360
	}

	// HSV back to RGB
	c2 := v * s
	x := c2 * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c2
	r2, g2, b2 := 0.0, 0.0, 0.0
	switch {
	case h < 60:
		r2, g2, b2 = c2, x, 0
	case h < 120:
		r2, g2, b2 = x, c2, 0
	case h < 180:
		r2, g2, b2 = 0, c2, x
	case h < 240:
		r2, g2, b2 = 0, x, c2
	case h < 300:
		r2, g2, b2 = x, 0, c2
	default:
		r2, g2, b2 = c2, 0, x
	}

	return color.RGBA{
		R: uint8((r2 + m) * 255),
		G: uint8((g2 + m) * 255),
		B: uint8((b2 + m) * 255),
		A: c.A, // preserve original alpha
	}
}

// =============================================================================
// Passage Mode Types
// =============================================================================

// passageWord represents a single word in the teleprompter view.
type passageWord struct {
	text      string // the word text
	globalIdx int    // index in the full passage word list
	typed     bool   // whether this word has been correctly typed
	skipped   bool   // whether this word was skipped (miss)
}

// passageLine represents a wrapped line of text in the passage.
type passageLine struct {
	words []passageWord
	width int     // measured pixel width
	y     float64 // Y position in text space (from top)
}

// passageState tracks all state for the teleprompter passage mode.
type passageState struct {
	lines           []passageLine // wrapped lines for current passage
	currentWordIdx  int           // global word index in the passage
	scrollOffset    float64       // current scroll position (pixels from top of text)
	targetScroll    float64       // target scroll for smooth interpolation
	scrollSpeed     float64       // current scroll speed (pixels/frame)
	transitionAlpha float64       // 0-1 crossfade alpha during transition
	transitionTimer int           // frames remaining in transition pause
	hueShift        float64       // additional hue shift during transition
	passageComplete bool          // whether current passage is done
	lastTypeTick    int           // tick of last successful keypress
	totalWords      int           // total words in current passage
	wordsTyped      int           // words completed (correct + skipped)
	lineHeight      float64       // computed line height
	passageNum      int           // which passage we're on
	initialized     bool          // whether passage state has been set up
	paused          bool          // waiting for first keypress
}

// =============================================================================
// Game Types
// =============================================================================

// Word represents a falling word that the player must type.
type Word struct {
	text      string
	x, y      float64
	prevY     float64
	vx, vy    float64
	paused    bool
	missCount int
	armed     bool   // bomb armed after first completion
	armTimer  int    // frames until bomb explodes
	driftPhase float64 // for orbital movement
}

// Game is the main game state.
type Game struct {
	words           []*Word
	typed           string
	score           int
	level           int
	wordsTotal      int
	spawnCD         int
	over            bool
	tick            int
	startTick       int
	particles       *ParticleSystem
	stars           *Starfield
	audio           *Audio
	slowCharges     int
	streak          int
	adaptSpeed      float64
	waitingForInput bool
	targetWordIdx   int
	missCount       int
	popTimer        int
	autoFire        bool
	beatTick        int
	gameMode        int    // 0=Random, 1=Passage
	passageIdx      int
	passageNum      int
	slowTimer       int
	passage         passageState
}

// =============================================================================
// Game Initialization
// =============================================================================

// NewGame creates a new game instance with initial state.
func NewGame() *Game {
	g := &Game{
		spawnCD:         120,
		waitingForInput: true,
			beatTick:        BeatFrames,
			startTick:       -1,
		particles:       NewParticleSystem(),
		stars:           NewStarfield(),
	}
	g.audio = g.initAudio()
	if g.audio != nil {
		g.audio.startMusic()
	}
	return g
}

// initAudio creates the audio system for the game.
func (g *Game) initAudio() *Audio {
	ctx := audio.NewContext(44100)
	if ctx == nil {
		return nil
	}
	return &Audio{
		ctx:       ctx,
		bass:      loadLoopData(ctx, bass2Data),
		synth:     loadLoopData(ctx, synthData),
		perc:      loadLoopData(ctx, percData),
		slowBass:  loadLoopData(ctx, bass1Data),
		hitData:   hitData,
		missData:  missData,
		clickData: clickData,
		powerData: powerupData,
	}
}

// =============================================================================
// Word Management
// =============================================================================

// currentWord returns the first word in the list (the one being typed).
func (g *Game) currentWord() *Word {
	if len(g.words) == 0 {
		return nil
	}
	return g.words[0]
}

// findBestWord finds the lowest word matching the typed prefix.
// Returns the word and its index, or nil if no match.
func (g *Game) findBestWord() *Word {
	if len(g.typed) == 0 {
		// No typed prefix — default to lowest word
		return g.currentWord()
	}
	var best *Word
	for _, w := range g.words {
		if len(w.text) >= len(g.typed) && w.text[:len(g.typed)] == g.typed {
			if best == nil || w.y > best.y {
				best = w
			}
		}
	}
	return best
}

// explodeBomb destroys all words within BombRadius
func (g *Game) explodeBomb(bomb *Word) {
	// Spawn explosion particles
	if g.particles != nil {
		for i := 0; i < 20; i++ {
			angle := float64(i) * 0.314
			dist := rand.Float64() * BombRadius
			g.particles.SpawnBurst(
				bomb.x+math.Cos(angle)*dist,
				bomb.y+math.Sin(angle)*dist,
				"boom",
			)
		}
	}

	// Find words in radius and remove them
	var remaining []*Word
	for _, w := range g.words {
		if w == bomb {
			continue  // skip the bomb itself
		}
		dx := w.x - bomb.x
		dy := w.y - bomb.y
		dist := math.Sqrt(dx*dx + dy*dy)
		if dist > BombRadius {
			remaining = append(remaining, w)
		}
		// Words in radius are destroyed (don't count towards WPM)
	}
	g.words = remaining

	// Remove the bomb itself
	for i, w := range g.words {
		if w == bomb {
			g.words = append(g.words[:i], g.words[i+1:]...)
			break
		}
	}
}

// spawnWord adds a new word to the screen based on the current game mode.
func (g *Game) spawnWord() {
	var w string
	if g.gameMode == 1 {
		// Passage mode: words come from current passage in order
		p := passages[g.passageNum%len(passages)]
		words := strings.Fields(p)
		if g.passageIdx < len(words) {
			w = words[g.passageIdx]
		} else {
			// Passage complete, move to next
			g.passageNum++
			g.passageIdx = 0
			p = passages[g.passageNum%len(passages)]
			words = strings.Fields(p)
			w = words[0]
		}
	} else {
		w = wordList[rand.IntN(len(wordList))]
	}

	// In passage mode, don't spawn duplicate words on screen
	if g.gameMode == 1 {
		for _, existing := range g.words {
			if existing.text == w {
				return
			}
		}
	}

	// Escape pod launches from ship at top
	shipX := float64(ScreenWidth) / 2
	shipY := 60.0
	angle := rand.Float64() * math.Pi * 0.6 + math.Pi * 0.2  // launch downward at angles
	speed := 1.0 + rand.Float64()*0.5
	g.words = append(g.words, &Word{
		text: w,
		x:    shipX + (rand.Float64()-0.5)*100,
		y:    shipY,
		vx:   math.Cos(angle) * speed,
		vy:   math.Sin(angle) * speed * 0.5,  // mostly downward
	})
}

// =============================================================================
// Difficulty System
// =============================================================================

// difficulty calculates the current speed and spawn gap based on level and word count.
func (g *Game) difficulty() (speed float64, gap int) {
	// Calculate base speed
	base := BaseSpeed + SpeedStep*float64(g.level)
	speed = base
	if speed > MaxSpeed {
		speed = MaxSpeed
	}

	// Adaptive spawn gap: target ~6 words on screen
	wordsOnScreen := 0
	for _, w := range g.words {
		if w.y > -20 && w.y < float64(ScreenHeight) {
			wordsOnScreen++
		}
	}
	targetWords := 6
	baseGap := MaxSpawnGap - g.level*2
	if baseGap < MinSpawnGap {
		baseGap = MinSpawnGap
	}

	// Adjust gap based on word density
	if wordsOnScreen > targetWords+2 {
		// Too many words — slow spawning
		gap = baseGap + (wordsOnScreen-targetWords)*20
	} else if wordsOnScreen < targetWords-2 {
		// Too few words — speed up spawning
		gap = baseGap - (targetWords-wordsOnScreen)*10
		if gap < MinSpawnGap {
			gap = MinSpawnGap
		}
	} else {
		gap = baseGap
	}
	return
}

// =============================================================================
// Passage Mode Methods
// =============================================================================

// wrapPassage breaks passage text into lines that fit the screen width.
func (g *Game) wrapPassage(text string) []passageLine {
	maxWidth := ScreenWidth - PassageMargin*2
	words := strings.Fields(text)
	var lines []passageLine
	var currentWords []passageWord
	var currentWidth int
	lineIdx := 0

	for i, w := range words {
		wordBounds := font.MeasureString(passageFont, w)
		wordWidth := wordBounds.Round()

		// Check if adding this word would exceed the line width
		extraWidth := 0
		if currentWidth > 0 {
			spaceBounds := font.MeasureString(passageFont, " ")
			extraWidth = spaceBounds.Round()
		}

		if currentWidth > 0 && currentWidth+extraWidth+wordWidth > maxWidth {
			// Wrap to next line
			lines = append(lines, passageLine{
				words: currentWords,
				width: currentWidth,
				y:     float64(lineIdx * PassageLineHeight),
			})
			lineIdx++
			currentWords = nil
			currentWidth = 0
		}

		if currentWidth > 0 {
			spaceBounds := font.MeasureString(passageFont, " ")
			currentWidth += spaceBounds.Round()
		}

		currentWords = append(currentWords, passageWord{
			text:      strings.ToLower(w),
			globalIdx: i,
		})
		currentWidth += wordWidth
	}

	if len(currentWords) > 0 {
		lines = append(lines, passageLine{
			words: currentWords,
			width: currentWidth,
			y:     float64(lineIdx * PassageLineHeight),
		})
	}

	return lines
}

// initPassageMode sets up the passage state for a new passage.
func (g *Game) initPassageMode() {
	g.passageNum = 0
	g.passage.currentWordIdx = 0
	g.passage.scrollOffset = 0
	g.passage.targetScroll = 0
	g.passage.scrollSpeed = PassageScrollBase
	g.passage.transitionAlpha = 1.0
	g.passage.transitionTimer = 60 // 1 second pause before first passage
	g.passage.hueShift = 0
	g.passage.passageComplete = false
	g.passage.lastTypeTick = 0
	g.passage.wordsTyped = 0
	g.passage.lineHeight = PassageLineHeight
	g.passage.passageNum = 0
	g.passage.paused = true
	g.passage.initialized = true
	g.setupPassageLines()
}

// setupPassageLines wraps the current passage into lines and resets word state.
func (g *Game) setupPassageLines() {
	p := passages[g.passageNum % len(passages)]
	g.passage.lines = g.wrapPassage(p)
	g.passage.totalWords = len(strings.Fields(p))
	g.passage.currentWordIdx = 0
	g.passage.wordsTyped = 0
	g.passage.passageComplete = false
	g.passage.scrollOffset = 0
	g.passage.targetScroll = 0
	g.passage.transitionAlpha = 1.0
	g.passage.transitionTimer = 60
}

// passageCurrentLine returns the line index containing the current word.
func (g *Game) passageCurrentLine() int {
	idx := 0
	for li, line := range g.passage.lines {
		for _, w := range line.words {
			if w.globalIdx == g.passage.currentWordIdx {
				return li
			}
		}
		idx = li
	}
	return idx
}

// updatePassageMode advances the teleprompter scroll and handles transitions.
func (g *Game) updatePassageMode() {
	// Handle transition pause
	if g.passage.transitionTimer > 0 {
		g.passage.transitionTimer--
		g.passage.transitionAlpha = float64(g.passage.transitionTimer) / 60.0
		if g.passage.transitionTimer == 0 {
			g.passage.transitionAlpha = 1.0
		}
		return
	}

	// Calculate target scroll to center the current line
	currentLine := g.passageCurrentLine()
	lineY := g.passage.lines[currentLine].y
	g.passage.targetScroll = lineY - float64(ScreenHeight/2 - PassageLineHeight/2)

	// Scroll speed tied to typing speed
	if g.tick - g.passage.lastTypeTick < 120 {
		// Player is actively typing - scroll faster
		g.passage.scrollSpeed = PassageScrollBase * 2.0
	} else if g.tick - g.passage.lastTypeTick < 300 {
		// Player slowed down
		g.passage.scrollSpeed = PassageScrollBase * 1.2
	} else {
		// Player stopped - slow scroll or pause
		g.passage.scrollSpeed = PassageScrollBase * 0.3
	}

	// Smooth scroll interpolation
	if g.passage.scrollOffset < g.passage.targetScroll {
		g.passage.scrollOffset += g.passage.scrollSpeed
		if g.passage.scrollOffset > g.passage.targetScroll {
			g.passage.scrollOffset = g.passage.targetScroll
		}
	} else if g.passage.scrollOffset > g.passage.targetScroll {
		g.passage.scrollOffset -= g.passage.scrollSpeed
		if g.passage.scrollOffset < g.passage.targetScroll {
			g.passage.scrollOffset = g.passage.targetScroll
		}
	}
}

// advancePassageWord moves to the next word in the passage (correct or skip).
func (g *Game) advancePassageWord(skipped bool) {
	// Find and mark the current word
	for li := range g.passage.lines {
		for wi := range g.passage.lines[li].words {
			if g.passage.lines[li].words[wi].globalIdx == g.passage.currentWordIdx {
				if skipped {
					g.passage.lines[li].words[wi].skipped = true
				} else {
					g.passage.lines[li].words[wi].typed = true
				}
			}
		}
	}

	g.passage.currentWordIdx++
	g.passage.wordsTyped++
	g.passage.lastTypeTick = g.tick

	// Check passage completion
	if g.passage.currentWordIdx >= g.passage.totalWords {
		g.passage.passageComplete = true
		// Move to next passage after transition
		g.passageNum++
		g.passage.transitionTimer = 90 // 1.5 second transition
		g.passage.hueShift += 30       // shift hue for new passage
		g.setupPassageLines()
	}
}

// =============================================================================
// Game Update Logic
// =============================================================================

// Update handles one frame of game logic.
func (g *Game) Update() error {
	// --- Game Over State ---
	if g.over {
		g.particles.Update()
		return g.handleGameOverInput()
	}

	// --- Active Game ---
	g.tick++
	g.particles.Update()

	// Passage mode has its own update loop
	if g.gameMode == 1 {
		if !g.passage.initialized {
			g.initPassageMode()
		}
		g.updatePassageMode()
		g.handlePassageInput()
		return nil
	}

	g.stars.Update()

	// Update music volumes based on level
	g.updateMusicVolumes()

	// Power-up timer countdown
	if g.slowTimer > 0 {
		g.slowTimer--
	}

	// Beat tracking
	g.beatTick--
	onBeat := g.beatTick <= 0
	if onBeat {
		g.beatTick = BeatFrames
	}

	_, gap := g.difficulty()

	// Adaptive spawn cooldown
	g.spawnCD--

	// Spawn management
	if g.waitingForInput && len(g.words) > 0 {
		// Wait - first word already on screen
	} else if g.slowTimer > 0 {
		// Pause spawning during power-up
	} else if g.spawnCD <= 0 && onBeat {
		g.spawnWord()
		g.spawnCD = gap
	}

	// Bomb timer countdown
	for _, w := range g.words {
		if w.armed {
			w.armTimer--
			if w.armTimer <= 0 {
				g.explodeBomb(w)
			}
		}
	}

	// Pop animation timer
	if g.popTimer > 0 {
		g.popTimer--
	}

	// Update word positions and check for game over
	for _, w := range g.words {
		w.prevY = w.y
		if !w.paused {
			// Gravity direction without acceleration
		dx := PlanetX - w.x
		dy := PlanetY - w.y
		dist := math.Sqrt(dx*dx + dy*dy)
		if dist > PlanetRadius {
			// Normalize direction and apply constant speed toward planet
			speed := 0.3
			w.vx = (dx / dist) * speed
			w.vy = (dy / dist) * speed
		}

		// Apply velocity
		w.x += w.vx
		w.y += w.vy

		// Wrap around sides
		if w.x < -100 {
			w.x = float64(ScreenWidth + 50)
		}
		if w.x > float64(ScreenWidth + 100) {
			w.x = -50
		}
	}
		if w.y > float64(ScreenHeight) {
			g.over = true
			if g.audio != nil {
				g.audio.stopMusic()
			}
			return nil
		}
	}

	// Handle typing input
	g.handleInput()

	// Space activates power-up (during gameplay only)
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) && g.slowCharges > 0 && g.slowTimer <= 0 {
		g.slowCharges--
		g.slowTimer = SlowDuration
		if g.audio != nil {
			g.audio.playPowerup()
		}
	}

	// Escape triggers game over
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.over = true
		if g.audio != nil {
			g.audio.stopMusic()
		}
	}
	return nil
}

// updateMusicVolumes adjusts music volumes based on current level.
func (g *Game) updateMusicVolumes() {
	if g.audio == nil {
		return
	}

	bassVol := float64(g.level-1) * 0.06
	if bassVol > 0.5 {
		bassVol = 0.5
	}
	percVol := float64(g.level-1) * 0.04
	if percVol > 0.4 {
		percVol = 0.4
	}

	// Swap bass during slow time
	if g.slowTimer > 0 {
		g.audio.setBassVolume(0)
		g.audio.setSlowBassVolume(bassVol)
	} else {
		g.audio.setBassVolume(bassVol)
		g.audio.setSlowBassVolume(0)
	}
	g.audio.setPercVolume(percVol)
}

// handleGameOverInput processes input when the game is over.
// Returns ebiten.Termination if the player presses Escape to quit.
func (g *Game) handleGameOverInput() error {
	// Mode switching
	if inpututil.IsKeyJustPressed(ebiten.Key1) {
		g.gameMode = 0
	}
	if inpututil.IsKeyJustPressed(ebiten.Key2) {
		g.gameMode = 1
	}

	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		if g.audio != nil {
			g.audio.playClick()
		}
		// Reset game state while preserving shared resources
		*g = Game{
			score:           0,
			level:           1,
			spawnCD:         120,
			waitingForInput: true,
			beatTick:        BeatFrames,
			startTick:       -1,
			adaptSpeed:      1.0,
			gameMode:        g.gameMode,
			particles:       g.particles,
			stars:           g.stars,
			audio:           g.audio,
		}
		// Initialize passage mode if switching to it
		if g.gameMode == 1 {
			g.initPassageMode()
		}
		if g.audio != nil {
			g.audio.startMusic()
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}
	return nil
}

// =============================================================================
// Input Handling
// =============================================================================

// handleInput processes typing input during active gameplay.
func (g *Game) handleInput() {
	if g.over {
		return
	}

	keys := inpututil.AppendJustPressedKeys(nil)
	for _, k := range keys {
		s := k.String()

		// Handle special keys
		if len(s) != 1 {
			if k == ebiten.KeyBackspace && len(g.typed) > 0 {
				g.typed = g.typed[:len(g.typed)-1]
			}
			// Tab toggles auto-fire mode
			if k == ebiten.KeyTab {
				g.autoFire = !g.autoFire
			}
			continue
		}

		cur := g.findBestWord()
		if cur == nil {
			continue
		}

		ch := strings.ToLower(s)
		g.waitingForInput = false
		if g.startTick == -1 {
			g.startTick = g.tick
		}
		for _, w := range g.words {
			w.paused = false
		}

		if len(g.typed) < len(cur.text) && ch == string(cur.text[len(g.typed)]) {
			// Correct letter typed
			g.typed += ch
			g.popTimer = 6  // frames of pop animation
			if g.audio != nil {
				g.audio.playHit()
			}
			// Word completed
			if g.typed == cur.text {
				g.particles.SpawnBurst(cur.x+float64(len(cur.text))*3.5, cur.y, cur.text)
				// Remove the completed word
				for i, w := range g.words {
					if w == cur {
						g.words = append(g.words[:i], g.words[i+1:]...)
						break
					}
				}
				// Score based on word length: 3-4=1pt, 5-6=3pt, 7+=5pt
				scoreAdd := 1
				if len(cur.text) >= 7 {
					scoreAdd = 5
				} else if len(cur.text) >= 5 {
					scoreAdd = 3
				}
				g.score += scoreAdd
				g.wordsTotal++
				g.typed = ""
				g.level = g.wordsTotal/5 + 1
				if g.audio != nil { g.audio.playLevelup() }
				if g.gameMode == 1 {
					g.passageIdx++
				}
				// Grant slow charge every 10 words
				if g.wordsTotal%10 == 0 {
					g.slowCharges++
				}
			}
		} else {
			// Check if ANY word matches this prefix
			found := false
			for _, w := range g.words {
				if w != cur && len(w.text) >= len(g.typed)+1 && w.text[:len(g.typed)+1] == g.typed+ch {
					found = true
					break
				}
			}
			if !found {
				// True miss — no word matches
				g.missCount++
				if g.audio != nil {
					g.missCount++
				g.audio.playMiss()
				}
				g.typed = ""
			}
		}
	}
}

// handlePassageInput processes typing input in passage mode.
// In passage mode, mistypes skip the current word.
func (g *Game) handlePassageInput() {
	if g.over || g.passage.transitionTimer > 0 {
		return
	}

	// Initialize passage lines if needed
	if len(g.passage.lines) == 0 {
		g.setupPassageLines()
	}

	// Find the current word text
	currentWord := ""
	for li := range g.passage.lines {
		for _, w := range g.passage.lines[li].words {
			if w.globalIdx == g.passage.currentWordIdx {
				currentWord = w.text
				break
			}
		}
		if currentWord != "" {
			break
		}
	}
	if currentWord == "" {
		return
	}

	keys := inpututil.AppendJustPressedKeys(nil)
	for _, k := range keys {
		s := k.String()

		// Handle special keys
		if len(s) != 1 {
			if k == ebiten.KeyEscape {
				g.over = true
				if g.audio != nil {
					g.audio.stopMusic()
				}
				return
			}
			continue
		}

		// First keypress starts the game
		if g.passage.paused {
			g.passage.paused = false
			g.waitingForInput = false
			if g.startTick == -1 {
				g.startTick = g.tick
			}
		}

		ch := strings.ToLower(s)

		// Check if this is the correct next character
		typedSoFar := ""
		for li := range g.passage.lines {
			for _, w := range g.passage.lines[li].words {
				if w.globalIdx == g.passage.currentWordIdx {
					// Count characters of typed words before this word
					for li2 := range g.passage.lines {
						for _, w2 := range g.passage.lines[li2].words {
							if w2.typed && w2.globalIdx < g.passage.currentWordIdx {
								typedSoFar += w2.text + " "
							}
						}
					}
					break
				}
			}
			if typedSoFar != "" {
				break
			}
		}

		// Get what the player has typed of the current word
		// Simple approach: track typed characters within the current word
		wordTypedLen := 0
		for li := range g.passage.lines {
			for _, w := range g.passage.lines[li].words {
				if w.globalIdx == g.passage.currentWordIdx {
					// We need to know how many chars typed
					// Use g.typed to track this
					wordTypedLen = len(g.typed)
					break
				}
			}
			if wordTypedLen > 0 || len(g.typed) > 0 {
				break
			}
		}

		if len(g.typed) < len(currentWord) && ch == string(currentWord[len(g.typed)]) {
			// Correct letter typed
			g.typed += ch
			g.popTimer = 6  // frames of pop animation
			g.popTimer = 6
			if g.audio != nil {
				g.audio.playHit()
			}
			// Word completed?
			if g.typed == currentWord {
				// Score based on word length: 3-4=1pt, 5-6=3pt, 7+=5pt
				scoreAdd := 1
				if len(currentWord) >= 7 {
					scoreAdd = 5
				} else if len(currentWord) >= 5 {
					scoreAdd = 3
				}
				g.score += scoreAdd
				g.wordsTotal++
				g.typed = ""
				g.level = g.wordsTotal/5 + 1
				if g.audio != nil { g.audio.playLevelup() }
				g.advancePassageWord(false)
				if g.wordsTotal%10 == 0 {
					g.slowCharges++
				}
			}
		} else {
			// Wrong letter - skip this word
			if g.audio != nil {
				g.missCount++
				g.audio.playMiss()
			}
			g.typed = ""
			g.score++
			g.wordsTotal++
			g.level = g.wordsTotal/5 + 1
				if g.audio != nil { g.audio.playLevelup() }
			g.advancePassageWord(true) // skip=true
		}
	}
}

// =============================================================================
// Drawing Functions
// =============================================================================

// Draw renders the current game frame.
func (g *Game) Draw(screen *ebiten.Image) {
	levelHue = float64(g.level) * 20.0

	if g.gameMode == 1 {
		g.drawPassageMode(screen)
		return
	}

	// Random mode rendering
	screen.Fill(hueRotate(ColorBg, levelHue))
	g.stars.Draw(screen)

	if g.over {
		g.particles.Draw(screen)
		g.drawOver(screen)
		return
	}

	g.drawAtmosphere(screen)
	g.drawWords(screen)
	g.drawLCARS(screen)
	g.drawTarget(screen)
	g.particles.Draw(screen)
}

// drawAtmosphere renders the danger gradient at the bottom of the screen.
func (g *Game) drawAtmosphere(screen *ebiten.Image) {
	gradientStart := float64(DangerLine)
	gradientHeight := float64(ScreenHeight - DangerLine)
	for y := 0; y < int(gradientHeight); y++ {
		progress := float64(y) / gradientHeight
		alpha := uint8(progress * progress * progress * 120)
		r := uint8(progress * 200)
		gn := uint8(progress * 80)
		b := uint8(progress * 10)
		c := color.RGBA{r, gn, b, alpha}
		for x := 0; x < ScreenWidth; x += 4 {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(4, 1)
			op.GeoM.Translate(float64(x), gradientStart+float64(y))
			op.ColorScale.ScaleWithColor(c)
			screen.DrawImage(g.particles.circle, op)
		}
	}
}

// drawTarget renders the current target word at the top of the screen.
func (g *Game) drawTarget(screen *ebiten.Image) {
	cur := g.currentWord()
	if cur == nil {
		waitBounds := font.MeasureString(targetFont, "waiting...")
		waitX := (ScreenWidth - waitBounds.Round()) / 2
		text.Draw(screen, "waiting...", targetFont, waitX, WordY, hueRotate(ColorDim, levelHue))
		return
	}

	// Calculate total width using actual glyph advances
	totalW := 0
	for _, ch := range cur.text {
		advance, _ := targetFont.GlyphAdvance(ch)
		totalW += advance.Round()
	}
	startX := (ScreenWidth - totalW) / 2

	// Draw each character with appropriate coloring
	px := startX
	for i, ch := range cur.text {
		var c color.RGBA
		if i < len(g.typed) {
			c = ColorTyped
		} else if i == len(g.typed) {
			c = ColorNext
		} else {
			c = ColorDim
		}
		// Apply pop animation to current letter
		charY := WordY + 8
		if i == len(g.typed) && g.popTimer > 0 {
			charY -= g.popTimer  // Pop up effect
		}
		text.Draw(screen, string(ch), targetFont, px, charY, c)
		advance, _ := targetFont.GlyphAdvance(ch)
		px += advance.Round()
	}
}

// drawWords renders all falling words to the screen.
func (g *Game) drawWords(screen *ebiten.Image) {
	for _, w := range g.words {
		// Danger glow effect when word approaches danger line
		if w.y > float64(DangerLine-30) {
			proximity := (w.y - float64(DangerLine-30)) / 130.0
			if proximity > 1 {
				proximity = 1
			}
			glowAlpha := uint8(proximity * 100)
			for dx := -2; dx < len(w.text)*7+4; dx += 2 {
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(1, 2)
				op.GeoM.Translate(w.x+float64(dx)-1, w.y-8)
				op.ColorScale.ScaleWithColor(color.RGBA{255, 60, 60, glowAlpha})
				screen.DrawImage(g.particles.circle, op)
			}
		}

		// Draw shadow
		text.Draw(screen, w.text, gameFont, int(w.x)+1, int(w.y)+1, color.RGBA{0, 0, 0, 180})

		// Draw word with typing progress coloring
		if g.currentWord() == w && len(g.typed) > 0 {
			fallPx := int(w.x)
			for ci, ch := range w.text {
				var c color.RGBA
				if ci < len(g.typed) {
					c = ColorTyped
				} else {
					c = ColorNext
				}
				text.Draw(screen, string(ch), gameFont, fallPx, int(w.y), c)
				advance, _ := gameFont.GlyphAdvance(ch)
				fallPx += advance.Round()
			}
		} else {
			wordColor := hueRotate(ColorWhite, levelHue)
			if w.armed {
				// Armed bomb - pulse red
				pulse := math.Sin(float64(g.tick)*0.3) * 0.5 + 0.5
				wordColor = color.RGBA{
					R: uint8(255),
					G: uint8(50 + pulse*100),
					B: 0,
					A: 255,
				}
			} else if w.missCount > 0 {
				// Dim word based on misses
				alpha := uint8(255 - w.missCount*60)
				wordColor = color.RGBA{255, 200, 200, alpha}
			}
			text.Draw(screen, w.text, gameFont, int(w.x), int(w.y), wordColor)
		}
	}
}

// drawLCARS renders the LCARS-style HUD elements.
func (g *Game) drawLCARS(screen *ebiten.Image) {
	// --- Top Bar ---
	// Left corner block
	for y := 5; y < 12; y++ {
		for x := 5; x < 15; x++ {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(1, 1)
			op.GeoM.Translate(float64(x), float64(y))
			op.ColorScale.ScaleWithColor(hueRotate(ColorOrange, levelHue))
			screen.DrawImage(g.particles.circle, op)
		}
	}
	// Top horizontal line
	for x := 15; x < ScreenWidth-100; x += 2 {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(2, 1)
		op.GeoM.Translate(float64(x), 5)
		op.ColorScale.ScaleWithColor(hueRotate(ColorOrange, levelHue))
		screen.DrawImage(g.particles.circle, op)
	}
	// Right corner block
	for y := 5; y < 12; y++ {
		for x := ScreenWidth - 100; x < ScreenWidth-90; x++ {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(1, 1)
			op.GeoM.Translate(float64(x), float64(y))
			op.ColorScale.ScaleWithColor(hueRotate(ColorPurple, levelHue))
			screen.DrawImage(g.particles.circle, op)
		}
	}

	// --- Score Display ---
	modeStr := "RND"
	if g.gameMode == 1 {
		modeStr = "PAS"
	}
	text.Draw(screen, modeStr, hudFont, 20, 50, hueRotate(ColorDim, levelHue))
	text.Draw(screen, "SCORE", hudFont, 20, 30, hueRotate(ColorOrange, levelHue))
	// Score formula: 1 point per 3 letters in word (min 1)
	text.Draw(screen, fmt.Sprintf("%d", g.score), hudFont, 70, 30, hueRotate(ColorWhite, levelHue))

	// --- Level Display (centered) ---
	lvlStr := fmt.Sprintf("LVL %d", g.level)
	lvlBounds := font.MeasureString(hudFont, lvlStr)
	lvlX := (ScreenWidth - lvlBounds.Round()) / 2
	text.Draw(screen, lvlStr, hudFont, lvlX, 30, hueRotate(ColorLavender, levelHue))

	// --- Slow Charges / Active Timer ---
	if g.slowTimer > 0 {
		secs := g.slowTimer / 60
		text.Draw(screen, fmt.Sprintf("SPECIAL %ds", secs), hudFont, ScreenWidth/2+60, 30, hueRotate(ColorTyped, levelHue))
	} else if g.slowCharges > 0 {
		text.Draw(screen, fmt.Sprintf("SPECIAL x%d", g.slowCharges), hudFont, ScreenWidth/2+60, 30, hueRotate(ColorGold, levelHue))
	}

	// --- WPM Display ---
	wpm := 0
	if g.startTick >= 0 && (g.tick - g.startTick) >= 3600 {
		wpm = int(float64(g.wordsTotal) * 3600.0 / float64(g.tick - g.startTick))
	}
	if g.gameMode == 1 {
		p := passages[g.passageNum%len(passages)]
		totalWords := len(strings.Fields(p))
		text.Draw(screen, fmt.Sprintf("P%d %d/%d", g.passageNum+1, g.passageIdx, totalWords), hudFont, ScreenWidth-110, 50, hueRotate(ColorLavender, levelHue))
	}
	if wpm > 0 {
		text.Draw(screen, fmt.Sprintf("WPM: %d", wpm), hudFont, ScreenWidth-110, 30, hueRotate(ColorGold, levelHue))
	} else {
		text.Draw(screen, "WPM: --", hudFont, ScreenWidth-110, 30, hueRotate(ColorDim, levelHue))
	}
	text.Draw(screen, fmt.Sprintf("MISS: %d", g.missCount), hudFont, ScreenWidth-110, 55, hueRotate(ColorRedOrange, levelHue))

	// --- Bottom Bar ---
	for x := 5; x < ScreenWidth-5; x += 2 {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(2, 1)
		op.GeoM.Translate(float64(x), float64(ScreenHeight-10))
		op.ColorScale.ScaleWithColor(color.RGBA{255, 100, 50, 80})
		screen.DrawImage(g.particles.circle, op)
	}

	// Help text
	text.Draw(screen, "SPACE: use special", hudFont, ScreenWidth/2-80, ScreenHeight-18, hueRotate(ColorDim, levelHue))
}

// drawOver renders the game over screen.
func (g *Game) drawOver(screen *ebiten.Image) {
	// Center "RESCUE FAILED"
	hbBounds := font.MeasureString(targetFont, "RESCUE FAILED")
	hbX := (ScreenWidth - hbBounds.Round()) / 2
	text.Draw(screen, "RESCUE FAILED", targetFont, hbX, ScreenHeight/2-40, hueRotate(ColorDanger, levelHue))

	// Current mode
	modeStr := "RANDOM"
	if g.gameMode == 1 {
		modeStr = "PASSAGE"
	}
	modeBounds := font.MeasureString(hudFont, modeStr)
	modeX := (ScreenWidth - modeBounds.Round()) / 2
	text.Draw(screen, modeStr, hudFont, modeX, ScreenHeight/2-15, hueRotate(ColorLavender, levelHue))

	// Mode selection help
	modeSelBounds := font.MeasureString(hudFont, "1: Random  2: Passage")
	modeSelX := (ScreenWidth - modeSelBounds.Round()) / 2
	text.Draw(screen, "1: Random  2: Passage", hudFont, modeSelX, ScreenHeight/2+10, hueRotate(ColorDim, levelHue))

	// Final stats
	finalWPM := 0
	if g.startTick >= 0 && (g.tick - g.startTick) >= 3600 {
		finalWPM = int(float64(g.wordsTotal) * 3600.0 / float64(g.tick - g.startTick))
	}
	scoreStr := fmt.Sprintf("score: %d  WPM: %d  level: %d", g.score, finalWPM, g.level)
	scoreBounds := font.MeasureString(hudFont, scoreStr)
	scoreX := (ScreenWidth - scoreBounds.Round()) / 2
	text.Draw(screen, scoreStr, hudFont, scoreX, ScreenHeight/2+35, hueRotate(ColorWhite, levelHue))

	// Play prompt
	playBounds := font.MeasureString(hudFont, "press SPACE to play")
	playX := (ScreenWidth - playBounds.Round()) / 2
	text.Draw(screen, "press SPACE to play", hudFont, playX, ScreenHeight/2+55, hueRotate(ColorPurple, levelHue))
}


// drawPassageMode renders the full-screen teleprompter passage view.
func (g *Game) drawPassageMode(screen *ebiten.Image) {
	// Background - no starfield, just the dark background
	screen.Fill(hueRotate(ColorBg, levelHue+g.passage.hueShift))

	if g.over {
		g.particles.Draw(screen)
		g.drawOver(screen)
		return
	}

	// Draw passage text
	g.drawPassage(screen)

	// Draw minimal HUD
	g.drawPassageHUD(screen)

	g.particles.Draw(screen)
}

// drawPassage renders the scrolling passage text.
func (g *Game) drawPassage(screen *ebiten.Image) {
	if len(g.passage.lines) == 0 {
		return
	}

	centerY := float64(ScreenHeight / 2)
	marginX := float64(PassageMargin)

	for _, line := range g.passage.lines {
		// Calculate screen Y position for this line
		lineScreenY := line.y - g.passage.scrollOffset + centerY

		// Skip lines that are off-screen
		if lineScreenY < -float64(PassageLineHeight*2) || lineScreenY > float64(ScreenHeight+PassageLineHeight) {
			continue
		}

		// Calculate fade alpha based on distance from center
		distance := math.Abs(lineScreenY-centerY) / float64(ScreenHeight/2)
		if distance > 1.0 {
			distance = 1.0
		}
		alpha := 1.0 - distance*distance
		if alpha < 0.15 {
			alpha = 0.15
		}

		// Transition fade
		if g.passage.transitionAlpha < 1.0 {
			alpha *= g.passage.transitionAlpha
		}

		// Draw each word in the line
		px := marginX
		for _, word := range line.words {
			var wordColor color.RGBA
			var drawAlpha float64

			if word.typed {
				// Typed words: dimmed
				wordColor = ColorDim
				drawAlpha = alpha * 0.5
			} else if word.skipped {
				// Skipped words: red with strikethrough
				wordColor = ColorDanger
				drawAlpha = alpha * 0.7
			} else if word.globalIdx == g.passage.currentWordIdx {
				// Current word: bright gold, character-by-character coloring
				g.drawPassageWord(screen, word.text, px, lineScreenY, alpha, true)
				px += g.wordAdvance(word.text)
				// Add space
				spaceBounds := font.MeasureString(passageFont, " ")
				px += float64(spaceBounds.Round())
				continue
			} else if word.globalIdx > g.passage.currentWordIdx {
				// Upcoming words: medium brightness
				wordColor = ColorWhite
				drawAlpha = alpha * 0.6
			} else {
				// Already passed (shouldn't happen with our tracking, but fallback)
				wordColor = ColorDim
				drawAlpha = alpha * 0.3
			}

			// Draw the word with computed color/alpha
			c := wordColor
			c.A = uint8(drawAlpha * 255)
			text.Draw(screen, word.text, passageFont, int(px), int(lineScreenY), c)

			// Draw strikethrough for skipped words
			if word.skipped {
				wordW := font.MeasureString(passageFont, word.text)
				strikeY := lineScreenY - 8
				for dx := 0; dx < wordW.Round(); dx += 2 {
					op := &ebiten.DrawImageOptions{}
					op.GeoM.Scale(2, 1)
					op.GeoM.Translate(px+float64(dx), strikeY)
					op.ColorScale.ScaleWithColor(color.RGBA{255, 60, 60, uint8(drawAlpha * 200)})
					screen.DrawImage(g.particles.circle, op)
				}
			}

			wordBounds := font.MeasureString(passageFont, word.text)
			px += float64(wordBounds.Round())

			// Add space between words
			spaceBounds := font.MeasureString(passageFont, " ")
			px += float64(spaceBounds.Round())
		}
	}
}

// drawPassageWord draws a single word with per-character coloring for the current word.
func (g *Game) drawPassageWord(screen *ebiten.Image, word string, x, y, alpha float64, isCurrent bool) {
	px := x
	for i, ch := range word {
		var c color.RGBA
		if i < len(g.typed) {
			// Typed character: orange accent
			c = ColorOrange
		} else if i == len(g.typed) {
			// Current character: bright gold (cursor-like)
			c = ColorGold
		} else {
			// Untyped characters in current word: white
			c = ColorWhite
		}
		c.A = uint8(alpha * 255)
		text.Draw(screen, string(ch), passageFont, int(px), int(y), c)
		advance, _ := passageFont.GlyphAdvance(ch)
		px += float64(advance.Round())
	}
}

// wordAdvance returns the pixel width of a word rendered in passageFont.
func (g *Game) wordAdvance(word string) float64 {
	return float64(font.MeasureString(passageFont, word).Round())
}

// drawPassageHUD renders the minimal HUD for passage mode.
func (g *Game) drawPassageHUD(screen *ebiten.Image) {
	// WPM display - top left
	wpm := 0
	if g.startTick >= 0 && (g.tick-g.startTick) >= 3600 {
		wpm = int(float64(g.wordsTotal) * 3600.0 / float64(g.tick-g.startTick))
	}
	if wpm > 0 {
		text.Draw(screen, fmt.Sprintf("WPM %d", wpm), hudFont, 20, 30, hueRotate(ColorGold, levelHue))
	} else {
		text.Draw(screen, "WPM --", hudFont, 20, 30, hueRotate(ColorDim, levelHue))
	}

	// Passage progress - top right
	passProgress := fmt.Sprintf("P%d %d/%d", g.passageNum+1, g.passage.wordsTyped, g.passage.totalWords)
	ppBounds := font.MeasureString(hudFont, passProgress)
	px := ScreenWidth - ppBounds.Round() - 20
	text.Draw(screen, passProgress, hudFont, px, 30, hueRotate(ColorLavender, levelHue))

	// Score - top center
	scoreStr := fmt.Sprintf("%d", g.score)
	sBounds := font.MeasureString(hudFont, scoreStr)
	sx := (ScreenWidth - sBounds.Round()) / 2
	text.Draw(screen, scoreStr, hudFont, sx, 30, hueRotate(ColorWhite, levelHue))

	// Mode indicator - small
	text.Draw(screen, "PASSAGE", hudFont, 20, 50, hueRotate(ColorDim, levelHue))

	// Help text - bottom
	text.Draw(screen, "ESC: quit", hudFont, ScreenWidth/2-60, ScreenHeight-18, hueRotate(ColorDim, levelHue))
}

// =============================================================================
// Layout
// =============================================================================

// Layout returns the game's screen dimensions.
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return ScreenWidth, ScreenHeight
}

// =============================================================================
// Main
// =============================================================================

func main() {
	loadFont()
	ebiten.SetWindowSize(ScreenWidth, ScreenHeight)
	ebiten.SetWindowTitle("Stardate Type Drop")
	ebiten.SetTPS(FrameRate)

	if err := ebiten.RunGame(NewGame()); err != nil {
		panic(err)
	}
}

// =============================================================================
// Passages
// =============================================================================

var passages = []string{
	"the enterprise warped into the nebula at maximum speed sensors detected an unknown energy signature emanating from the central core of the gaseous cloud",
	"captain picard ordered the shields raised as the ship penetrated deeper into the anomaly the crew braced for potential hostilities unknown to starfleet",
	"lieutenant worf reported unusual readings on the tactical display the energy patterns were unlike anything in the starfleet database a first contact scenario",
	"doctor crusher prepared the sickbay for potential casualties while commander data analyzed the strange signals for any linguistic patterns that might indicate communication",
	"the prime directive was clear observe and document without interference but the developing civilization below was already reaching for the stars with primitive rockets",
}
