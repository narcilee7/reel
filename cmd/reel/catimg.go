package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/narcilee7/reel/pkg/reel"
)

// runCatImg implements `reel img [-width N] <image>`.
func runCatImg(args []string) error {
	fs := flag.NewFlagSet("img", flag.ContinueOnError)
	width := fs.Int("width", 0, "max image width in terminal cells")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: reel img [-width N] <image>")
	}

	engine := reel.New(reel.WithAutoDetect(), reel.WithMaxWidth(*width))
	content, err := reel.LoadImage(fs.Arg(0))
	if err != nil {
		return err
	}
	return engine.Render(os.Stdout, content)
}
