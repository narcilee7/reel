// Package content implements the Content interface for concrete source types
// (images, Markdown documents, and a reserved chart type).
package content

import (
	"image"

	"github.com/narcilee7/reel/pkg/reel/protocol"
)

// Image is a raster image.
type Image struct {
	Image image.Image
	Alt   string
}

func (c *Image) Kind() protocol.ContentKind { return protocol.ContentKindImage }

// ToIR wraps the image in a single ImageFragment.
func (c *Image) ToIR(ctx protocol.Context) (*protocol.IntermediateRep, error) {
	return &protocol.IntermediateRep{Fragments: []protocol.Fragment{
		&protocol.ImageFragment{Image: c.Image, Alt: c.Alt},
	}}, nil
}
