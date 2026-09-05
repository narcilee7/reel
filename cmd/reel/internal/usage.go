// Package internal provides helpers private to the reel CLI.
package internal

import (
	"fmt"
	"io"
)

// PrintUsage writes the CLI usage message to w.
func PrintUsage(w io.Writer) {
	fmt.Fprintln(w, `reel — terminal capability-aware rendering pipeline

Usage:
  reel [flags] <file>       render an image, PDF or markdown file (auto-detected)
  reel img [flags] <image>  render an image (PNG/JPEG/GIF, animated on capable terminals)
  reel md  [flags] <file>   render a markdown document
  reel read  [-width N] <file.md>
                            interactive markdown reader: lazy image loading,
                            scroll-out deletion, link navigation (n/p, enter to open)
  reel gallery [dir]        interactive image browser: thumbnail grid, keyboard
                            navigation, fullscreen view (enter, esc to go back)
  reel probe [-json|-template]
                            print terminal detection diagnostics for calibration

Flags:
  -width N   max image width in terminal cells (default: no limit)`)
}
