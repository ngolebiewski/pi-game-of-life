// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2015 Martin Lindhe
// SPDX-FileCopyrightText: 2016 The Ebitengine Authors
//
// The original project is gol (https://github.com/martinlindhe/gol) by Martin Lindhe.
// This file modifies that example to:
//  - run fullscreen
//  - use a configurable CellSize (default 32)
//  - slowly shift through rainbow hues per generation
//  - fade out dead cells over 5 generations (ghost trail)
//  - handle restart on any input
//
// Modified by: Nick Golebiewski and ChatGPT

package main

import (
	"image/color"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// ------------------------ Config ------------------------

const (
	CellSize            = 32
	InitialLiveFraction = 0.50
	FadeGenerations     = 5 // how many generations ghosts persist
)

// ------------------------ Helpers ------------------------

// hsvToRGB converts hue-saturation-value (HSV) to RGB.
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

// ------------------------ World ------------------------

type World struct {
	live       []bool  // live cells
	fadeLevels []uint8 // how many generations since death (0–FadeGenerations)
	width      int
	height     int
}

// NewWorld creates a new world.
func NewWorld(width, height int, maxInitLiveCells int) *World {
	w := &World{
		live:       make([]bool, width*height),
		fadeLevels: make([]uint8, width*height),
		width:      width,
		height:     height,
	}
	w.init(maxInitLiveCells)
	return w
}

func (w *World) init(maxLiveCells int) {
	if maxLiveCells <= 0 {
		maxLiveCells = int(float64(w.width*w.height) * InitialLiveFraction)
	}
	for i := 0; i < len(w.live); i++ {
		w.live[i] = false
		w.fadeLevels[i] = 0
	}
	for i := 0; i < maxLiveCells; i++ {
		x := rand.Intn(w.width)
		y := rand.Intn(w.height)
		w.live[y*w.width+x] = true
	}
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
	width := w.width
	height := w.height
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
					nextFade[i] = 1 // start fade
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
}

func (w *World) Draw(pix []byte, hue float64) {
	for i, alive := range w.live {
		var r, g, b uint8
		if alive {
			r, g, b = hsvToRGB(hue, 1, 1)
		} else if w.fadeLevels[i] > 0 {
			alpha := 1 - float64(w.fadeLevels[i])/float64(FadeGenerations)
			r, g, b = hsvToRGB(hue-30, 0.8, alpha) // offset hue slightly for ghost color
		} else {
			r, g, b = 0, 0, 0
		}
		pix[4*i] = r
		pix[4*i+1] = g
		pix[4*i+2] = b
		pix[4*i+3] = 0xff
	}
}

// ------------------------ Game ------------------------

type Game struct {
	world        *World
	worldImage   *ebiten.Image
	worldPixels  []byte
	lastW, lastH int
	lastUpdate   time.Time
	hue          float64
}

func (g *Game) Update() error {
	// Quit on Q or ESC
	if ebiten.IsKeyPressed(ebiten.KeyQ) || ebiten.IsKeyPressed(ebiten.KeyEscape) {
		return ebiten.Termination
	}

	// Restart on any input
	mouseClicked := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) ||
		ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) ||
		ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle)
	touches := ebiten.AppendTouchIDs(nil)
	keys := ebiten.AppendInputChars(nil)
	if mouseClicked || len(touches) > 0 || len(keys) > 0 {
		if g.world != nil {
			g.world = NewWorld(g.lastW, g.lastH, 0)
		}
	}

	const updatesPerSecond = 8
	delay := time.Second / updatesPerSecond
	if time.Since(g.lastUpdate) < delay {
		return nil
	}
	g.lastUpdate = time.Now()

	g.hue += 2 // slowly rotate through rainbow hues
	g.world.Update()
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.world == nil || g.worldImage == nil {
		screen.Fill(color.Black)
		return
	}
	g.world.Draw(g.worldPixels, g.hue)
	g.worldImage.WritePixels(g.worldPixels)
	screen.Fill(color.Black)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(CellSize), float64(CellSize))
	op.Filter = ebiten.FilterNearest
	screen.DrawImage(g.worldImage, op)
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
		g.world = NewWorld(worldW, worldH, 0)
		g.worldImage = ebiten.NewImage(worldW, worldH)
		g.worldPixels = make([]byte, 4*worldW*worldH)
		g.lastW = worldW
		g.lastH = worldH
	}
	return outsideWidth, outsideHeight
}

func main() {
	rand.Seed(time.Now().UnixNano())
	g := &Game{}
	ebiten.SetFullscreen(true)
	ebiten.SetWindowTitle("Rainbow Life — Ebiten Experimental")
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
