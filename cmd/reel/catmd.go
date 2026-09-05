package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/narcilee7/reel/pkg/reel"
)

// runCatMd implements `reel md [-width N] <file>`.
func runCatMd(args []string) error {
	fs := flag.NewFlagSet("md", flag.ContinueOnError)
	width := fs.Int("width", 0, "max image width in terminal cells")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: reel md [-width N] <file>")
	}

	engine := reel.New(reel.WithAutoDetect(), reel.WithMaxWidth(*width))
	content, err := reel.LoadMarkdown(fs.Arg(0))
	if err != nil {
		return err
	}
	return engine.Render(os.Stdout, content)
}
