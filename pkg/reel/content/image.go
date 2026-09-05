// Package content implements the Content interface for concrete source types
// (images, Markdown documents, charts and PDF previews).
package content

import (
	"image"
	"time"

	"github.com/narcilee7/reel/pkg/reel/protocol"
)

// Image is a raster image, optionally animated. When Frames holds more than
// one frame (Frames[0] always equals Image), ToIR produces an
// AnimationFragment instead of an ImageFragment; Source/SourceMIME carry the
// original file bytes for protocols that embed native files (iTerm2 GIF).
type Image struct {
	Image      image.Image
	Alt        string
	Frames     []image.Image   // non-empty for multi-frame content
	Delay      []time.Duration // aligned with Frames
	Source     []byte          // optional original file bytes
	SourceMIME string          // MIME type of Source
}

func (c *Image) Kind() protocol.ContentKind { return protocol.ContentKindImage }

// ToIR wraps the image in a single fragment: an AnimationFragment for
// multi-frame content, an ImageFragment otherwise (the static path is
// identical to Phase 1/2).
func (c *Image) ToIR(ctx protocol.Context) (*protocol.IntermediateRep, error) {
	if len(c.Frames) > 1 {
		return &protocol.IntermediateRep{Fragments: []protocol.Fragment{
			&protocol.AnimationFragment{
				Image:      c.Image,
				Alt:        c.Alt,
				Frames:     c.Frames,
				Delay:      c.Delay,
				Source:     c.Source,
				SourceMIME: c.SourceMIME,
			}}}, nil
	}
	return &protocol.IntermediateRep{Fragments: []protocol.Fragment{
		&protocol.ImageFragment{Image: c.Image, Alt: c.Alt},
	}}, nil
}
