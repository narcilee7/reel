// Package tui provides Bubble Tea components that embed Reel-rendered
// content in terminal user interfaces.
//
// Boundary: unlike cmd/reel, which may only use the root SDK API, this
// package ships with the SDK and may import the internal protocol and layout
// packages. Every lifecycle operation it relies on — image deletion,
// refitting, geometry reprobing — is also exposed as public API on
// reel.Engine, so third parties can implement the same lifecycle without
// importing pkg/tui (see docs/PHASE2.md §4.1).
//
// Concurrency: reel.Engine and reel.Document are not safe for concurrent
// use. A TUI program holds one Engine and one live Document, driven from the
// single Bubble Tea model goroutine, which matches this package's design.
//
// Scrolling: in Kitty terminals images are anchored to cells and scroll with
// the grid (managed mode) at zero handling cost. In iTerm2/Sixel/AnsiArt
// terminals images are anchored to absolute screen coordinates and would
// leave artifacts (snapshot mode), so images are hidden while scrolling and
// restored 150ms after scrolling stops. The TUI image experience is designed
// for Kitty; other protocols are graceful degradation.
package tui
