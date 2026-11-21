package sticker

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Watermark is the maker's mark drawn on every sticker Joy makes, so a sticker
// that leaves this host stays identifiable next to whoever made it.
const Watermark = "JOYX"

// drawWatermark puts a small translucent mark in the bottom-right corner of a
// finished sticker canvas. It uses the same face and outline as meme text,
// only smaller and quieter, so it never covers the picture.
func drawWatermark(canvas *image.RGBA) error {
	parsed, err := loadMemeFont()
	if err != nil {
		return fmt.Errorf("load watermark font: %w", err)
	}
	size := max(16, Size/18)
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return fmt.Errorf("create watermark face: %w", err)
	}
	defer func() { _ = face.Close() }()

	padding := Size / 24
	width := font.MeasureString(face, Watermark)
	left := fixed.I(Size-padding) - width
	baseline := Size - padding
	origin := fixed.Point26_6{X: left, Y: fixed.I(baseline - face.Metrics().Descent.Ceil())}

	radius := int(math.Round(float64(size) / 16))
	drawer := &font.Drawer{Dst: canvas, Face: face}
	for angle := 0.0; angle < 2*math.Pi; angle += math.Pi / 8 {
		drawer.Src = image.NewUniform(color.RGBA{A: 220})
		drawer.Dot = origin.Add(fixed.Point26_6{
			X: fixed.I(int(math.Round(float64(radius) * math.Cos(angle)))),
			Y: fixed.I(int(math.Round(float64(radius) * math.Sin(angle)))),
		})
		drawer.DrawString(Watermark)
	}
	drawer.Src = image.NewUniform(color.RGBA{R: 255, G: 255, B: 255, A: 235})
	drawer.Dot = origin
	drawer.DrawString(Watermark)
	return nil
}
