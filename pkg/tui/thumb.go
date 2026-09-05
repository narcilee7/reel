package tui

import (
	"image"

	"github.com/narcilee7/reel/pkg/reel/encoder"
)

// Thumbnail scales img to a cols×rows cell rectangle at cellW×cellH pixels
// per cell. It is the public Presentation-layer wrapper around
// encoder.BoxResize: cmd/reel may not import the encoder package, but third
// parties using only the SDK surface can build the same galleries through
// this helper.
func Thumbnail(img image.Image, cellW, cellH, cols, rows int) image.Image {
	return encoder.BoxResize(img, cellW*cols, cellH*rows)
}
