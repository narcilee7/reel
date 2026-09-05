package protocol

import (
	"fmt"
	"io"
	"strings"

	"github.com/narcilee7/reel/pkg/reel/detector"
	"github.com/narcilee7/reel/pkg/reel/encoder"
)

// sixelMaxColors is the default palette size. Terminals advertise a larger
// budget via detector's ReportedMaxColors in a future phase.
const sixelMaxColors = 256

type sixelProtocol struct{}

func (p *sixelProtocol) Name() string { return "sixel" }

func (p *sixelProtocol) Detect(env *detector.Environment) (SupportLevel, error) {
	term := env.Get("TERM")
	if strings.Contains(term, "sixel") || term == "foot" || term == "mlterm" || term == "yaft" {
		return detector.SupportStatic, nil
	}
	return detector.SupportNone, nil
}

func (p *sixelProtocol) Capabilities() Capabilities {
	return Capabilities{}
}

func (p *sixelProtocol) Write(w io.Writer, ir *IntermediateRep, opts *RenderOptions) error {
	for _, f := range ir.Fragments {
		var err error
		switch t := f.(type) {
		case *TextFragment:
			err = writeText(w, t)
		case *ImageFragment:
			err = p.writeImage(w, t, opts)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// writeImage downsampled the image to the fragment's cell rectangle at the
// terminal's cell pixel size (sixel display size equals pixel size) and emits
// it as a sixel DCS sequence, restoring the cursor afterwards.
func (p *sixelProtocol) writeImage(w io.Writer, f *ImageFragment, opts *RenderOptions) error {
	if f.Image.Bounds().Empty() {
		return writeText(w, &TextFragment{Text: "[image: " + f.Alt + "]\n"})
	}

	img := f.Image
	if f.Rect.Width > 0 && f.Rect.Height > 0 && opts != nil && opts.CellSize.Width > 0 && opts.CellSize.Height > 0 {
		img = encoder.BoxResize(f.Image, f.Rect.Width*opts.CellSize.Width, f.Rect.Height*opts.CellSize.Height)
	}

	payload, err := encoder.EncodeSixel(img, sixelMaxColors)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\x1bP0;1;0q%s\x1b\\", payload); err != nil {
		return err
	}
	// Return the cursor to a fresh line below the image for following text.
	_, err = fmt.Fprint(w, "\r\n")
	return err
}
