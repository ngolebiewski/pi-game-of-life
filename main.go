package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	CellSize         = 32
	DefaultFraction  = 0.33
	MaxTrails        = 100
	FadeGenerations  = 5
	UpdatesPerSecond = 8
	KeyCooldownMs    = 100
	MinFraction      = 0.1
	MaxFraction      = 0.9
	FractionStep     = 0.05
)

func hsvToRGB(h, s, v float64) (r, g, b uint8) {
	h = math.Mod(h, 360)
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c
	var r1, g1, b1 float64
	switch {
	case h < 60:
		r1, g1, b1 = c, x, 0
	case h < 120:
		r1, g1, b1 = x, c, 0
	case h < 180:
		r1, g1, b1 = 0, c, x
	case h < 240:
		r1, g1, b1 = 0, x, c
	case h < 300:
		r1, g1, b1 = x, 0, c
	default:
		r1, g1, b1 = c, 0, x
	}
	r = uint8((r1 + m) * 255)
	g = uint8((g1 + m) * 255)
	b = uint8((b1 + m) * 255)
	return
}

type World struct {
	live       []bool
	fadeLevels []uint8
	width      int
	height     int
	gen        int
}

func NewWorld(width, height int, fraction float64) *World {
	w := &World{
		live:       make([]bool, width*height),
		fadeLevels: make([]uint8, width*height),
		width:      width,
		height:     height,
		gen:        0,
	}
	w.init(fraction)
	return w
}

func (w *World) init(fraction float64) {
	if fraction <= 0 {
		fraction = DefaultFraction
	}
	maxLive := int(float64(w.width*w.height) * fraction)
	for i := range w.live {
		w.live[i] = false
		w.fadeLevels[i] = 0
	}
	for i := 0; i < maxLive; i++ {
		x := rand.Intn(w.width)
		y := rand.Intn(w.height)
		w.live[y*w.width+x] = true
	}
	w.gen = 0
}

func neighbourCount(a []bool, width, height, x, y int) int {
	c := 0
	for j := -1; j <= 1; j++ {
		for i := -1; i <= 1; i++ {
			if i == 0 && j == 0 {
				continue
			}
			x2 := x + i
			y2 := y + j
			if x2 < 0 || y2 < 0 || width <= x2 || height <= y2 {
				continue
			}
			if a[y2*width+x2] {
				c++
			}
		}
	}
	return c
}

func (w *World) Update() {
	width, height := w.width, w.height
	next := make([]bool, width*height)
	nextFade := make([]uint8, width*height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := y*width + x
			pop := neighbourCount(w.live, width, height, x, y)
			alive := w.live[i]

			if alive {
				if pop == 2 || pop == 3 {
					next[i] = true
					nextFade[i] = 0
				} else {
					next[i] = false
					nextFade[i] = 1
				}
			} else {
				if pop == 3 {
					next[i] = true
					nextFade[i] = 0
				} else if w.fadeLevels[i] > 0 && w.fadeLevels[i] < FadeGenerations {
					next[i] = false
					nextFade[i] = w.fadeLevels[i] + 1
				}
			}
		}
	}
	w.live = next
	w.fadeLevels = nextFade
	w.gen++
}

func (w *World) Draw(pix []byte, hue float64, trails int, whiteOnly bool) {
	for i, alive := range w.live {
		var r, g, b uint8
		alpha := uint8(255)

		if alive {
			if whiteOnly {
				r, g, b = 255, 255, 255
			} else {
				r, g, b = hsvToRGB(hue, 1, 1)
			}
		} else if w.fadeLevels[i] > 0 && trails > 0 {
			a := 1.0 - float64(w.fadeLevels[i])/float64(trails)
			if a < 0 {
				a = 0
			}
			alpha = uint8(a * 255)
			if whiteOnly {
				r, g, b = 255, 255, 255
			} else {
				r, g, b = hsvToRGB(hue-30, 0.8, a)
			}
		} else {
			r, g, b = 0, 0, 0
		}

		pix[4*i] = r
		pix[4*i+1] = g
		pix[4*i+2] = b
		pix[4*i+3] = alpha
	}
}

type Game struct {
	world        *World
	worldImage   *ebiten.Image
	worldPixels  []byte
	lastW, lastH int
	lastUpdate   time.Time
	hue          float64
	trails       int
	whiteOnly    bool
	paused       bool
	lastKeyTime  time.Time
	initFraction float64
}

func (g *Game) Update() error {
	now := time.Now()
	cooldown := now.Sub(g.lastKeyTime).Milliseconds() > KeyCooldownMs

	ids := ebiten.GamepadIDs()

	// ===== Reset (no cooldown) =====
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) ||
		ebiten.IsKeyPressed(ebiten.KeySpace) ||
		(len(ids) > 0 && ebiten.IsGamepadButtonPressed(ids[0], 1)) {
		g.world = NewWorld(g.lastW, g.lastH, g.initFraction)
		// don't update lastKeyTime here
	}

	// Quit
	if ebiten.IsKeyPressed(ebiten.KeyQ) || ebiten.IsKeyPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}

	// ===== Pausable & Adjustable actions (with cooldown) =====
	if cooldown {
		// Pause toggle
		if ebiten.IsKeyPressed(ebiten.KeyP) || (len(ids) > 0 && ebiten.IsGamepadButtonPressed(ids[0], 9)) {
			g.paused = !g.paused
			g.lastKeyTime = now
		}

		// White/Rainbow toggle with cooldown
		if cooldown && (ebiten.IsKeyPressed(ebiten.KeyEnter) || (len(ids) > 0 && ebiten.IsGamepadButtonPressed(ids[0], 8))) {
			g.whiteOnly = !g.whiteOnly
			g.lastKeyTime = now
		}

		// Trails adjustments
		if ebiten.IsKeyPressed(ebiten.KeyLeft) {
			g.trails--
			if g.trails < 0 {
				g.trails = 0
			}
			g.lastKeyTime = now
		}
		if ebiten.IsKeyPressed(ebiten.KeyRight) {
			g.trails++
			if g.trails > MaxTrails {
				g.trails = MaxTrails
			}
			g.lastKeyTime = now
		}

		// Fraction adjustments
		if ebiten.IsKeyPressed(ebiten.KeyUp) {
			g.initFraction += FractionStep
			if g.initFraction > MaxFraction {
				g.initFraction = MaxFraction
			}
			g.lastKeyTime = now
		}
		if ebiten.IsKeyPressed(ebiten.KeyDown) {
			g.initFraction -= FractionStep
			if g.initFraction < MinFraction {
				g.initFraction = MinFraction
			}
			g.lastKeyTime = now
		}

		// Controller axes for trails (optional)
		if len(ids) > 0 {
			id := ids[0]
			axisX := ebiten.GamepadAxisValue(id, 0)
			if axisX < -0.5 {
				g.trails--
				if g.trails < 0 {
					g.trails = 0
				}
				g.lastKeyTime = now
			} else if axisX > 0.5 {
				g.trails++
				if g.trails > MaxTrails {
					g.trails = MaxTrails
				}
				g.lastKeyTime = now
			}
		}
	}

	// Controller axes for fraction adjustments (with cooldown)
	if len(ids) > 0 && cooldown {
		id := ids[0]
		axisY := ebiten.GamepadAxisValue(id, 1)
		if axisY < -0.5 {
			g.initFraction += FractionStep
			if g.initFraction > MaxFraction {
				g.initFraction = MaxFraction
			}
			g.lastKeyTime = now
		} else if axisY > 0.5 {
			g.initFraction -= FractionStep
			if g.initFraction < MinFraction {
				g.initFraction = MinFraction
			}
			g.lastKeyTime = now
		}
	}

	// ===== Update world =====
	delay := time.Second / UpdatesPerSecond
	if time.Since(g.lastUpdate) < delay {
		return nil
	}
	g.lastUpdate = time.Now()

	if !g.paused {
		g.hue += 2
		g.world.Update()
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.world == nil || g.worldImage == nil {
		screen.Fill(color.Black)
		return
	}
	g.world.Draw(g.worldPixels, g.hue, g.trails, g.whiteOnly)
	g.worldImage.WritePixels(g.worldPixels)

	screen.Fill(color.Black)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(CellSize), float64(CellSize))
	op.Filter = ebiten.FilterNearest
	screen.DrawImage(g.worldImage, op)

	mode := "RAINBOW"
	if g.whiteOnly {
		mode = "WHITE"
	}
	txt := fmt.Sprintf("Trails: %d | Mode: %s | Generation: %d | Fraction: %.2f | %s", g.trails, mode, g.world.gen, g.initFraction, func() string {
		if g.paused {
			return "PAUSED"
		}
		return ""
	}())
	ebitenutil.DebugPrintAt(screen, txt, 10, screen.Bounds().Dy()-20)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	worldW := outsideWidth / CellSize
	worldH := outsideHeight / CellSize
	if worldW <= 0 {
		worldW = 1
	}
	if worldH <= 0 {
		worldH = 1
	}
	if g.world == nil || worldW != g.lastW || worldH != g.lastH {
		g.world = NewWorld(worldW, worldH, g.initFraction)
		g.worldImage = ebiten.NewImage(worldW, worldH)
		g.worldPixels = make([]byte, 4*worldW*worldH)
		g.lastW = worldW
		g.lastH = worldH
	}
	return outsideWidth, outsideHeight
}

func main() {
	rand.Seed(time.Now().UnixNano())
	g := &Game{
		initFraction: DefaultFraction,
	}
	ebiten.SetFullscreen(true)
	ebiten.SetWindowTitle("Rainbow Life — Ebiten Experimental")
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
