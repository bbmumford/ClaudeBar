package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

func main() {
	size := 64
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// Colors
	bgColor := color.RGBA{32, 33, 35, 255}       // Dark bg
	barBlue := color.RGBA{88, 140, 236, 255}      // Claude blue
	barYellow := color.RGBA{234, 179, 8, 255}     // Warning yellow
	barGreen := color.RGBA{74, 222, 128, 255}     // Low usage green

	// Fill with transparent
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, color.Transparent)
		}
	}

	// Draw rounded rectangle background
	radius := 12.0
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if inRoundedRect(float64(x), float64(y), 2, 2, float64(size-4), float64(size-4), radius) {
				img.Set(x, y, bgColor)
			}
		}
	}

	// Draw 3 vertical bars (usage chart icon)
	// Bar 1 (left, tall - blue)
	drawBar(img, 14, 16, 10, 32, barBlue)
	// Bar 2 (middle, medium - yellow)
	drawBar(img, 28, 24, 10, 24, barYellow)
	// Bar 3 (right, short - green)
	drawBar(img, 42, 32, 10, 16, barGreen)

	// Save icon
	dir := filepath.Join("assets", "icons")
	os.MkdirAll(dir, 0755)

	// Tray icon
	savePNG(img, filepath.Join(dir, "tray.png"))

	// App icon (same for now)
	savePNG(img, filepath.Join(dir, "app.png"))
}

func drawBar(img *image.RGBA, x, y, w, h int, c color.Color) {
	r := 3.0 // corner radius
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			px := float64(x + dx)
			py := float64(y + dy)
			if inRoundedRect(px, py, float64(x), float64(y), float64(w), float64(h), r) {
				img.Set(x+dx, y+dy, c)
			}
		}
	}
}

func inRoundedRect(px, py, rx, ry, rw, rh, radius float64) bool {
	if px < rx || px >= rx+rw || py < ry || py >= ry+rh {
		return false
	}

	// Check corners
	corners := [][2]float64{
		{rx + radius, ry + radius},             // top-left
		{rx + rw - radius, ry + radius},        // top-right
		{rx + radius, ry + rh - radius},        // bottom-left
		{rx + rw - radius, ry + rh - radius},   // bottom-right
	}

	for _, corner := range corners {
		cx, cy := corner[0], corner[1]
		// Check if point is in the corner region
		inCornerX := (px < rx+radius && cx == rx+radius) || (px >= rx+rw-radius && cx == rx+rw-radius)
		inCornerY := (py < ry+radius && cy == ry+radius) || (py >= ry+rh-radius && cy == ry+rh-radius)

		if inCornerX && inCornerY {
			dist := math.Sqrt((px-cx)*(px-cx) + (py-cy)*(py-cy))
			if dist > radius {
				return false
			}
		}
	}

	return true
}

func savePNG(img image.Image, path string) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	png.Encode(f, img)
}
