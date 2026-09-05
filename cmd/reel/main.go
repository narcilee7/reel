// Command reel renders images and Markdown files to the terminal.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/narcilee7/reel/cmd/reel/internal"
	"github.com/narcilee7/reel/pkg/reel"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "reel:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		internal.PrintUsage(os.Stderr)
		return fmt.Errorf("no input file")
	}

	switch args[0] {
	case "img", "catimg":
		return runCatImg(args[1:])
	case "md", "catmd":
		return runCatMd(args[1:])
	case "read":
		return runRead(args[1:])
	case "gallery":
		return runGallery(args[1:])
	case "probe":
		return runProbe(args[1:])
	case "help", "-h", "--help":
		internal.PrintUsage(os.Stdout)
		return nil
	}

	fs := flag.NewFlagSet("reel", flag.ContinueOnError)
	width := fs.Int("width", 0, "max image width in terminal cells")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		internal.PrintUsage(os.Stderr)
		return fmt.Errorf("expected exactly one file argument")
	}

	engine := reel.New(reel.WithAutoDetect(), reel.WithMaxWidth(*width))
	content, err := reel.Load(fs.Arg(0))
	if err != nil {
		return err
	}
	return engine.Render(os.Stdout, content)
}
