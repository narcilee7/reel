package reel

import (
	"github.com/narcilee7/reel/pkg/reel/detector"
)

type config struct {
	autoDetect bool
	profile    *detector.TerminalProfile
	maxWidth   int
	maxHeight  int
}

// Option configures Engine construction.
type Option func(*config)

// WithAutoDetect probes the terminal at Engine construction time.
func WithAutoDetect() Option {
	return func(c *config) { c.autoDetect = true }
}

// WithoutAutoDetect skips terminal probing; only explicitly hinted protocols
// and the Plain fallback are available.
func WithoutAutoDetect() Option {
	return func(c *config) { c.autoDetect = false }
}

// WithTerminalProfile pins the terminal profile, bypassing detection entirely.
func WithTerminalProfile(p *detector.TerminalProfile) Option {
	return func(c *config) { c.profile = p }
}

// WithMaxWidth limits rendered images to the given width in terminal cells.
func WithMaxWidth(cells int) Option {
	return func(c *config) { c.maxWidth = cells }
}

// WithMaxHeight limits rendered images to the given height in terminal cells.
func WithMaxHeight(cells int) Option {
	return func(c *config) { c.maxHeight = cells }
}
