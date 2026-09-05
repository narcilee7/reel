package protocol

import (
	"fmt"
	"image"
	"image/color"
	"io"

	"github.com/narcilee7/reel/pkg/reel/detector"
	"github.com/narcilee7/reel/pkg/reel/layout"
)

// ansiArtProtocol approximates images with Unicode half blocks (▀): the
// foreground color paints the upper half of a cell, the background color the
// lower half, giving two vertical pixels per cell in TrueColor terminals.
type ansiArtProtocol struct{}

func (p *ansiArtProtocol) Name() string { return "ansiart" }

func (p *ansiArtProtocol) Detect(env *detector.Environment) (SupportLevel, error) {
	return detector.SupportStatic, nil
}

func (p *ansiArtProtocol) Capabilities() Capabilities {
	return Capabilities{}
}

func (p *ansiArtProtocol) Write(w io.Writer, ir *IntermediateRep, opts *RenderOptions) error {
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

func (p *ansiArtProtocol) writeImage(w io.Writer, f *ImageFragment) error {
	img := f.Image
	b := img.Bounds()
	if b.Empty() {
		return writeText(w, &TextFragment{Text: "[image: " + f.Alt + "]\n"})
	}

	rect := f.Rect
	if rect.Width <= 0 || rect.Height <= 0 {
		rect.Width, rect.Height = 40, 20
	}

	// Each cell holds two vertical samples; cells are assumed ~1:2 aspect.
	for cy := 0; cy < rect.Height; cy++ {
		for cx := 0; cx < rect.Width; cx++ {
			top := sample(img, cx, cy*2, rect)
			bottom := sample(img, cx, cy*2+1, rect)
			if _, err := fmt.Fprintf(w, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀",
				top.R, top.G, top.B, bottom.R, bottom.G, bottom.B); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(w, "\x1b[0m\n"); err != nil {
			return err
		}
	}
	return nil
}

// sample reads the image color at cell-sample coordinates, mapping
// (sx, sy) in a rect.Width × rect.Height*2 grid onto the image.
func sample(img image.Image, sx, sy int, rect layout.CellRect) color.RGBA {
	b := img.Bounds()
	ix := sx * b.Dx() / rect.Width
	iy := sy * b.Dy() / (rect.Height * 2)
	if ix >= b.Dx() {
		ix = b.Dx() - 1
	}
	if iy >= b.Dy() {
		iy = b.Dy() - 1
	}
	return color.RGBAModel.Convert(img.At(b.Min.X+ix, b.Min.Y+iy)).(color.RGBA)
}
