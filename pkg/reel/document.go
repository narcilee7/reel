package reel

import (
	"bytes"
	"io"
	"strings"

	"github.com/narcilee7/reel/pkg/reel/detector"
	"github.com/narcilee7/reel/pkg/reel/layout"
	"github.com/narcilee7/reel/pkg/reel/protocol"
)

// Grid and CellRect are re-exported so SDK consumers can refit documents and
// build TUI layouts without importing the layout package.
type Grid = layout.Grid

// CellRect is a rectangle in terminal cell coordinates.
type CellRect = layout.CellRect

// Document is parsed and laid-out content that can be rendered repeatedly,
// as a unit. It backs one-shot rendering (Engine.Render) as well as the TUI
// lifecycle: re-fitting after a resize and deleting placed images.
//
// Document is not safe for concurrent use.
type Document struct {
	ir     *protocol.IntermediateRep
	engine *Engine
	imgIDs []uint32 // placement ids assigned by the last Render
}

// Prepare parses content into an IR and lays out image fragments on the
// engine's cell grid, without writing any bytes.
func (e *Engine) Prepare(c Content) (*Document, error) {
	ir, err := c.ToIR(protocol.Context{
		CellSize:   detector.Size{Width: e.grid.CellWidth, Height: e.grid.CellHeight},
		MaxCells:   detector.Size{Width: e.grid.Cols, Height: e.grid.Rows},
		ColorDepth: e.profile.ColorDepth,
	})
	if err != nil {
		return nil, err
	}

	fit := layout.FitOptions{MaxWidth: e.opts.MaxWidth, MaxHeight: e.opts.MaxHeight}
	for _, f := range ir.Fragments {
		// Fit only cares about the first frame's size; both image-class
		// fragment types share this code path.
		switch t := f.(type) {
		case *protocol.ImageFragment:
			if t.Rect.Width == 0 && t.Rect.Height == 0 {
				t.Rect = e.grid.Fit(t.Image, fit)
			}
		case *protocol.AnimationFragment:
			if t.Rect.Width == 0 && t.Rect.Height == 0 {
				t.Rect = e.grid.Fit(t.Image, fit)
			}
		}
	}
	return &Document{ir: ir, engine: e}, nil
}

// Render writes the escape sequence stream to w. Each call assigns the
// document's images a fresh run of kitty placement ids (recorded and exposed
// via ImageIDs), so callers can delete them later.
func (d *Document) Render(w io.Writer) error {
	n := 0
	for _, f := range d.ir.Fragments {
		if _, ok := protocol.StaticOf(f); ok {
			n++
		}
	}

	e := d.engine
	if n > 0 {
		e.opts.PlacementBase = e.placementID + 1
	} else {
		e.opts.PlacementBase = 0
	}
	if err := e.proto.Write(w, d.ir, &e.opts); err != nil {
		return err
	}

	if n > 0 {
		d.imgIDs = make([]uint32, n)
		for i := range d.imgIDs {
			d.imgIDs[i] = e.opts.PlacementBase + uint32(i)
		}
		e.placementID += uint32(n)
	}
	return nil
}

// ReFit recomputes the cell rectangles of all image fragments against a new
// grid, after a terminal resize (see Engine.Reprobe).
func (d *Document) ReFit(g *layout.Grid) {
	fit := layout.FitOptions{MaxWidth: d.engine.opts.MaxWidth, MaxHeight: d.engine.opts.MaxHeight}
	for _, f := range d.ir.Fragments {
		switch t := f.(type) {
		case *protocol.ImageFragment:
			t.Rect = g.Fit(t.Image, fit)
		case *protocol.AnimationFragment:
			t.Rect = g.Fit(t.Image, fit)
		}
	}
}

// ImageIDs returns the placement ids assigned by the last Render. The
// returned slice is a copy; it is empty before the first Render.
func (d *Document) ImageIDs() []uint32 {
	out := make([]uint32, len(d.imgIDs))
	copy(out, d.imgIDs)
	return out
}

// ImageRect returns the cell rectangle of the document's first image
// fragment (zero when the document contains no images).
func (d *Document) ImageRect() CellRect {
	for _, f := range d.ir.Fragments {
		if img, ok := protocol.StaticOf(f); ok {
			return img.Rect
		}
	}
	return CellRect{}
}

// RenderLines renders the document to w and returns its content as terminal
// rows for inline TUI embedding: text fragments contribute their lines, and
// image-class fragments occupy Rect.Height rows each (escape sequence on the
// first row, then blank rows). Joining the rows with "\n" yields the
// document's view; len(rows) is its visual height.
//
// The byte stream is identical to Render: fragments are rendered one at a
// time, which yields the same placement id sequence because each
// single-fragment write assigns its image the current PlacementBase.
func (d *Document) RenderLines(w io.Writer) ([]string, error) {
	e := d.engine
	n := 0
	for _, f := range d.ir.Fragments {
		if _, ok := protocol.StaticOf(f); ok {
			n++
		}
	}
	if n > 0 {
		e.opts.PlacementBase = e.placementID + 1
	} else {
		e.opts.PlacementBase = 0
	}

	var lines []string
	for _, f := range d.ir.Fragments {
		img, isImage := protocol.StaticOf(f)
		var buf bytes.Buffer
		if err := e.proto.Write(&buf, &protocol.IntermediateRep{Fragments: []protocol.Fragment{f}}, &e.opts); err != nil {
			return nil, err
		}
		fl := splitLines(buf.String())
		if isImage {
			for len(fl) < img.Rect.Height {
				fl = append(fl, "")
			}
		}
		lines = append(lines, fl...)
	}

	if n > 0 {
		d.imgIDs = make([]uint32, n)
		for i := range d.imgIDs {
			d.imgIDs[i] = e.opts.PlacementBase + uint32(i)
		}
		e.placementID += uint32(n)
	}
	return lines, nil
}

// splitLines splits s on newlines, dropping one trailing empty line produced
// by a final newline.
func splitLines(s string) []string {
	parts := strings.Split(s, "\n")
	if len(parts) > 1 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}
