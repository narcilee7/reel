package reel

import (
	"io"

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
type Engine struct {
	profile *detector.TerminalProfile
	proto   protocol.Protocol
	grid    *layout.Grid
	opts    protocol.RenderOptions
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

	proto := protocol.DefaultRegistry().Select(profile)

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
		},
	}
}

// Profile returns the cached terminal profile detected at construction.
func (e *Engine) Profile() *detector.TerminalProfile { return e.profile }

// ProtocolName returns the name of the protocol selected for this terminal.
func (e *Engine) ProtocolName() string { return e.proto.Name() }

// Render converts content to IR, lays out image fragments on the cell grid,
// and streams the selected protocol's escape sequences to w.
func (e *Engine) Render(w io.Writer, c Content) error {
	ir, err := c.ToIR(Context{})
	if err != nil {
		return err
	}

	fit := layout.FitOptions{MaxWidth: e.opts.MaxWidth, MaxHeight: e.opts.MaxHeight}
	for _, f := range ir.Fragments {
		if img, ok := f.(*protocol.ImageFragment); ok && img.Rect.Width == 0 && img.Rect.Height == 0 {
			img.Rect = e.grid.Fit(img.Image, fit)
		}
	}

	return e.proto.Write(w, ir, &e.opts)
}
