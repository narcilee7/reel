package reel

import (
	"io"
	"os"

	"github.com/narcilee7/reel/pkg/reel/detector"
	"github.com/narcilee7/reel/pkg/reel/layout"
	"github.com/narcilee7/reel/pkg/reel/protocol"
)

// Fallback cell geometry when ioctl cannot report pixel sizes.
const (
	defaultCellWidth  = 10
	defaultCellHeight = 20
)

// Engine is the rendering pipeline: it probes the terminal once at
// construction, then renders Content values as escape sequence streams.
//
// Engine is not safe for concurrent use; drive it from a single goroutine.
type Engine struct {
	profile     *detector.TerminalProfile
	proto       protocol.Protocol
	grid        *layout.Grid
	opts        protocol.RenderOptions
	placementID uint32
}

// New constructs an Engine. By default the terminal is auto-detected; use
// WithoutAutoDetect or WithTerminalProfile to override.
func New(opts ...Option) *Engine {
	cfg := config{autoDetect: true}
	for _, o := range opts {
		o(&cfg)
	}

	profile := cfg.profile
	if profile == nil && cfg.autoDetect {
		env := detector.EnvironmentFromOS()
		profile, _ = detector.NewDetector(env).Probe()
	}
	if profile == nil {
		profile = &detector.TerminalProfile{}
	}

	if os.Getenv("REEL_VERIFY_PROTOCOL") == "1" {
		profile = detector.VerifyProfile(profile)
	}

	proto := protocol.DefaultRegistry().Select(profile)
	if proto.Name() == "ansiart" && profile.ColorDepth > 0 && profile.ColorDepth <= 256 {
		proto = protocol.NewAnsiArt256()
	}

	grid := layout.NewGrid(
		profile.CellSize.Width, profile.CellSize.Height,
		profile.GridSize.Width, profile.GridSize.Height,
	)
	if grid.CellWidth <= 0 {
		grid.CellWidth = defaultCellWidth
	}
	if grid.CellHeight <= 0 {
		grid.CellHeight = defaultCellHeight
	}

	return &Engine{
		profile: profile,
		proto:   proto,
		grid:    grid,
		opts: protocol.RenderOptions{
			MaxWidth:  cfg.maxWidth,
			MaxHeight: cfg.maxHeight,
			CellSize:  detector.Size{Width: grid.CellWidth, Height: grid.CellHeight},
		},
	}
}

// Profile returns the cached terminal profile detected at construction.
func (e *Engine) Profile() *detector.TerminalProfile { return e.profile }

// ProtocolName returns the name of the protocol selected for this terminal.
func (e *Engine) ProtocolName() string { return e.proto.Name() }

// Grid returns the terminal cell grid the engine lays images out on.
func (e *Engine) Grid() *layout.Grid { return e.grid }

// Reprobe refreshes the terminal geometry (cell size and grid size) via
// ioctl. It is cheap (~0ms) and intended for TUI resize handling; protocol
// selection is not re-run because protocols do not change on resize.
func (e *Engine) Reprobe() error {
	cell, gridSize, err := detector.ReprobeGeometry()
	if err != nil {
		return err
	}
	if cell.Width > 0 && cell.Height > 0 {
		e.profile.CellSize = cell
		e.grid.CellWidth = cell.Width
		e.grid.CellHeight = cell.Height
	}
	if gridSize.Width > 0 && gridSize.Height > 0 {
		e.profile.GridSize = gridSize
		e.grid.Cols = gridSize.Width
		e.grid.Rows = gridSize.Height
	}
	e.opts.CellSize = detector.Size{Width: e.grid.CellWidth, Height: e.grid.CellHeight}
	return nil
}

// DeleteImages removes previously placed images with the given placement
// ids. It is a no-op (returning nil) when the selected protocol cannot
// delete images.
func (e *Engine) DeleteImages(w io.Writer, ids ...uint32) error {
	if d, ok := e.proto.(protocol.ImageDeleter); ok {
		return d.DeleteImages(w, ids...)
	}
	return nil
}

// Render converts content to IR, lays out image fragments on the cell grid,
// and streams the selected protocol's escape sequences to w.
func (e *Engine) Render(w io.Writer, c Content) error {
	doc, err := e.Prepare(c)
	if err != nil {
		return err
	}
	return doc.Render(w)
}
