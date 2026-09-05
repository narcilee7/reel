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
  reel [flags] <file>       render an image or markdown file (auto-detected)
  reel img [flags] <image>  render an image
  reel md  [flags] <file>   render a markdown document

Flags:
  -width N   max image width in terminal cells (default: no limit)`)
}
