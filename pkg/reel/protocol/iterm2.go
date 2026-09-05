package protocol

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"io"

	"github.com/narcilee7/reel/pkg/reel/detector"
	"github.com/narcilee7/reel/pkg/reel/layout"
)

type iterm2Protocol struct{}

func (p *iterm2Protocol) Name() string { return "iterm2" }

func (p *iterm2Protocol) Detect(env *detector.Environment) (SupportLevel, error) {
	if env.Get("TERM_PROGRAM") == "iTerm.app" {
		return detector.SupportStatic, nil
	}
	return detector.SupportNone, nil
}

func (p *iterm2Protocol) Capabilities() Capabilities {
	// iTerm2 buffers inline images in a single escape sequence, which
	// terminals typically cap around 1MB. Animation is supported natively by
	// embedding GIF file bytes (see writeAnimation).
	return Capabilities{Animation: true, MaxSizeBytes: 1 << 20}
}

func (p *iterm2Protocol) Write(w io.Writer, ir *IntermediateRep, opts *RenderOptions) error {
	imageIndex := 0
	for _, f := range ir.Fragments {
		var err error
		switch t := f.(type) {
		case *TextFragment:
			err = writeText(w, t)
		case *AnimationFragment:
			err = p.writeAnimation(w, t, opts, imageIndex)
			imageIndex++
		default:
			if img, ok := StaticOf(f); ok {
				err = p.writeImage(w, img, opts, imageIndex)
				imageIndex++
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// writeAnimation prefers iTerm2's native animation: when the fragment
// carries original GIF file bytes (RawSourcer), the file is embedded whole
// and iTerm2 plays it in a loop. Without raw bytes it degrades to the first
// frame's pixels.
func (p *iterm2Protocol) writeAnimation(w io.Writer, f *AnimationFragment, opts *RenderOptions, imageIndex int) error {
	if rs, ok := interface{}(f).(RawSourcer); ok {
		if raw, mime := rs.RawSource(); len(raw) > 0 && mime == "image/gif" {
			return p.writeEmbedded(w, raw, f.Rect, f.Image.Bounds())
		}
	}
	img, _ := StaticOf(f)
	return p.writeImage(w, img, opts, imageIndex)
}

// writeEmbedded emits an OSC 1337 File escape with raw file bytes inline.
func (p *iterm2Protocol) writeEmbedded(w io.Writer, raw []byte, rect layout.CellRect, imgBounds image.Rectangle) error {
	var dims string
	if rect.Width > 0 && rect.Height > 0 {
		dims = fmt.Sprintf("width=%dc;height=%dc;", rect.Width, rect.Height)
	} else {
		dims = fmt.Sprintf("width=%dpx;height=%dpx;", imgBounds.Dx(), imgBounds.Dy())
	}
	_, err := fmt.Fprintf(w, "\x1b]1337;File=inline=1;%s%s\x07", dims, base64.StdEncoding.EncodeToString(raw))
	return err
}

// writeImage emits a 1337 File escape with the whole image re-encoded as PNG.
// iTerm2 has no chunking, so the encoded image must fit in one sequence.
func (p *iterm2Protocol) writeImage(w io.Writer, f *ImageFragment, opts *RenderOptions, imageIndex int) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, f.Image); err != nil {
		return err
	}
	return p.writeEmbedded(w, buf.Bytes(), f.Rect, f.Image.Bounds())
}
