package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/narcilee7/reel/pkg/reel"
	"github.com/narcilee7/reel/pkg/reel/content"
	"github.com/narcilee7/reel/pkg/tui"
)

// runRead implements `reel read [-width N] <file.md>`: an interactive
// Markdown reader with lazy image placement and link navigation.
func runRead(args []string) error {
	fs := flag.NewFlagSet("read", flag.ContinueOnError)
	width := fs.Int("width", 0, "max image width in terminal cells")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: reel read [-width N] <file.md>")
	}

	src, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		return err
	}
	blocks := splitTopLevel(fs.Arg(0), src)
	if len(blocks) == 0 {
		return fmt.Errorf("%s: no content", fs.Arg(0))
	}

	engine := reel.New(reel.WithAutoDetect(), reel.WithMaxWidth(*width))
	pager := tui.NewPager(engine, blocks)
	app := &readApp{pager: pager}
	_, err = tea.NewProgram(app, tea.WithAltScreen()).Run()
	return err
}

// readApp is the Bubble Tea model wrapping the Pager: q quits, enter opens
// the link under the cursor in the system browser.
type readApp struct {
	pager *tui.Pager
}

func (a *readApp) Init() tea.Cmd { return a.pager.Init() }

func (a *readApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "q" {
		return a, tea.Quit
	}
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "enter" {
		if url, ok := a.pager.CurrentLink(); ok {
			openBrowser(url)
		}
	}
	_, cmd := a.pager.Update(msg)
	return a, cmd
}

func (a *readApp) View() string { return a.pager.View() }

// openBrowser best-effort opens url in the platform browser. Errors are
// ignored: a failed open must not tear down the TUI session.
func openBrowser(url string) {
	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("open", url)
	} else {
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// splitTopLevel cuts Markdown source at top-level headings (lines starting
// with "# ") into one block Content per section. Purely textual: no
// rendering happens here. A document without any top-level heading is a
// single block.
func splitTopLevel(path string, src []byte) []reel.Content {
	var blocks []reel.Content
	start := 0
	off := 0
	for _, line := range bytes.SplitAfter(src, []byte("\n")) {
		if off > start && len(line) > 0 && line[0] == '#' && (len(line) == 1 || line[1] == ' ') {
			blocks = append(blocks, content.NewMarkdown(path, src[start:off]))
			start = off
		}
		off += len(line)
	}
	if len(blocks) == 0 || start < len(src) {
		blocks = append(blocks, content.NewMarkdown(path, src[start:]))
	}
	return blocks
}
