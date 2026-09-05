package protocol

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"io"

	"github.com/narcilee7/reel/pkg/reel/detector"
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
	// terminals typically cap around 1MB.
	return Capabilities{MaxSizeBytes: 1 << 20}
}

func (p *iterm2Protocol) Write(w io.Writer, ir *IntermediateRep, opts *RenderOptions) error {
	for _, f := range ir.Fragments {
		var err error
		switch t := f.(type) {
		case *TextFragment:
			err = writeText(w, t)
		case *ImageFragment:
			err = p.writeImage(w, t)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// writeImage emits a 1337 File escape with the whole image inline as base64.
// iTerm2 has no chunking, so the encoded image must fit in one sequence.
func (p *iterm2Protocol) writeImage(w io.Writer, f *ImageFragment) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, f.Image); err != nil {
		return err
	}
	var dims string
	if f.Rect.Width > 0 && f.Rect.Height > 0 {
		dims = fmt.Sprintf("width=%dc;height=%dc;", f.Rect.Width, f.Rect.Height)
	} else {
		b := f.Image.Bounds()
		dims = fmt.Sprintf("width=%dpx;height=%dpx;", b.Dx(), b.Dy())
	}
	_, err := fmt.Fprintf(w, "\x1b]1337;File=inline=1;%s%s\x07", dims, base64.StdEncoding.EncodeToString(buf.Bytes()))
	return err
}
