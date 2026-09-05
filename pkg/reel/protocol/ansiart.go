package protocol

import (
	"fmt"
	"io"

	"github.com/narcilee7/reel/pkg/reel/detector"
	"github.com/narcilee7/reel/pkg/reel/encoder"
)

// ansiArtProtocol approximates images with Unicode half blocks (▀): the
// foreground color paints the upper half of a cell, the background color the
// lower half, giving two vertical pixels per cell in TrueColor terminals.
//
// ansiArtProtocol256 is the 256-color variant: identical sampling, but colors
// are quantized to the xterm palette and emitted as 38;5;n / 48;5;n. The
// Engine picks the variant matching the terminal's color depth.
type ansiArtProtocol struct{}

type ansiArtProtocol256 struct{}

func (p *ansiArtProtocol) Name() string { return "ansiart" }

func (p *ansiArtProtocol256) Name() string { return "ansiart" }

func (p *ansiArtProtocol) Detect(env *detector.Environment) (SupportLevel, error) {
	return detector.SupportStatic, nil
}

func (p *ansiArtProtocol256) Detect(env *detector.Environment) (SupportLevel, error) {
	return detector.SupportStatic, nil
}

func (p *ansiArtProtocol) Capabilities() Capabilities { return Capabilities{} }

func (p *ansiArtProtocol256) Capabilities() Capabilities { return Capabilities{} }

// NewAnsiArt256 returns the 256-color AnsiArt protocol variant. The Engine
// selects it for terminals whose color depth is known to be 256 or less; the
// truecolor variant is what DefaultRegistry contains.
func NewAnsiArt256() Protocol { return &ansiArtProtocol256{} }

func (p *ansiArtProtocol) Write(w io.Writer, ir *IntermediateRep, opts *RenderOptions) error {
	return writeAnsiArt(w, ir, opts, false)
}

func (p *ansiArtProtocol256) Write(w io.Writer, ir *IntermediateRep, opts *RenderOptions) error {
	return writeAnsiArt(w, ir, opts, true)
}

func writeAnsiArt(w io.Writer, ir *IntermediateRep, opts *RenderOptions, quantize bool) error {
	for _, f := range ir.Fragments {
		var err error
		switch t := f.(type) {
		case *TextFragment:
			err = writeText(w, t)
		case *AnimationFragment:
			img, _ := StaticOf(t)
			err = writeAnsiImage(w, img, opts, quantize)
		default:
			if img, ok := StaticOf(f); ok {
				err = writeAnsiImage(w, img, opts, quantize)
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// cellSizeOrDefault returns the terminal cell size from opts, falling back to
// the Phase 1 assumption of 1×2 (cell aspect 1:2) when unknown.
func cellSizeOrDefault(opts *RenderOptions) (int, int) {
	if opts != nil && opts.CellSize.Width > 0 && opts.CellSize.Height > 0 {
		return opts.CellSize.Width, opts.CellSize.Height
	}
	return 1, 2
}

func writeAnsiImage(w io.Writer, f *ImageFragment, opts *RenderOptions, quantize bool) error {
	b := f.Image.Bounds()
	if b.Empty() {
		return writeText(w, &TextFragment{Text: "[image: " + f.Alt + "]\n"})
	}

	rect := f.Rect
	if rect.Width <= 0 || rect.Height <= 0 {
		rect.Width, rect.Height = 40, 20
	}

	// Sample on a canvas at the terminal's real cell resolution: each cell
	// reads two points, the upper quarter (fg) and lower quarter (bg).
	cw, ch := cellSizeOrDefault(opts)
	canvas := encoder.BoxResize(f.Image, rect.Width*cw, rect.Height*ch)
	for cy := 0; cy < rect.Height; cy++ {
		for cx := 0; cx < rect.Width; cx++ {
			top := canvas.RGBAAt(cx*cw+cw/2, cy*ch+ch/4)
			bottom := canvas.RGBAAt(cx*cw+cw/2, cy*ch+3*ch/4)
			if quantize {
				if _, err := fmt.Fprintf(w, "\x1b[38;5;%dm\x1b[48;5;%dm▀",
					encoder.Quantize256(top), encoder.Quantize256(bottom)); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintf(w, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀",
					top.R, top.G, top.B, bottom.R, bottom.G, bottom.B); err != nil {
					return err
				}
			}
		}
		if _, err := fmt.Fprint(w, "\x1b[0m\n"); err != nil {
			return err
		}
	}
	return nil
}
