// Package detector probes the terminal emulator and caches a TerminalProfile,
// so protocol selection at render time is a plain table lookup.
package detector

import (
	"os"
	"strings"
	"sync"
)

// SupportLevel describes how fully a terminal supports an image protocol.
type SupportLevel int

const (
	SupportNone SupportLevel = iota
	SupportStatic
	SupportAnimation
	SupportNative
)

func (l SupportLevel) String() string {
	switch l {
	case SupportStatic:
		return "static"
	case SupportAnimation:
		return "animation"
	case SupportNative:
		return "native"
	default:
		return "none"
	}
}

// Size is a width/height pair, in pixels or in cells.
type Size struct {
	Width  int
	Height int
}

// ProtocolHint records that a terminal supports a named protocol at a level.
type ProtocolHint struct {
	Name  string
	Level SupportLevel
}

// TerminalProfile is the cached result of probing the terminal.
type TerminalProfile struct {
	Program    string // ghostty, kitty, iterm2, ...
	Protocols  []ProtocolHint
	CellSize   Size // cell pixel size
	GridSize   Size // rows and columns
	ColorDepth int  // 256 / 24-bit
	IsTTY      bool
}

// Supports reports whether the profile contains a hint for the named protocol.
func (p *TerminalProfile) Supports(name string) (ProtocolHint, bool) {
	for _, h := range p.Protocols {
		if h.Name == name {
			return h, true
		}
	}
	return ProtocolHint{}, false
}

// Environment is a snapshot of the process environment used for detection.
type Environment struct {
	Vars map[string]string
	TTY  bool
}

// Get returns the value of an environment variable.
func (e *Environment) Get(key string) string {
	return e.Vars[key]
}

// EnvironmentFromOS captures the current process environment.
func EnvironmentFromOS() *Environment {
	vars := make(map[string]string)
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			vars[kv[:i]] = kv[i+1:]
		}
	}
	_, _, _, _, err := winSizeOf(os.Stdout.Fd())
	return &Environment{Vars: vars, TTY: err == nil}
}

// Detector probes the terminal and produces a TerminalProfile.
type Detector struct {
	env *Environment
}

// NewDetector returns a Detector working from the given environment snapshot.
func NewDetector(env *Environment) *Detector {
	return &Detector{env: env}
}

type probeResult struct {
	program  string
	hints    []ProtocolHint
	cellSize Size
	gridSize Size
}

// Probe runs the zero-cost detection layers concurrently: L1 environment
// variables, L2 TERM heuristics and L4 ioctl window size.
func (d *Detector) Probe() (*TerminalProfile, error) {
	env := d.env

	results := make(chan probeResult, 3)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		results <- scanEnv(env)
	}()
	go func() {
		defer wg.Done()
		results <- matchTerm(env)
	}()
	go func() {
		defer wg.Done()
		results <- probeSystem()
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	merged := probeResult{}
	for r := range results {
		if merged.program == "" {
			merged.program = r.program
		}
		for _, h := range r.hints {
			addHint(&merged.hints, h.Name, h.Level)
		}
		if r.cellSize.Width > 0 {
			merged.cellSize = r.cellSize
		}
		if r.gridSize.Width > 0 {
			merged.gridSize = r.gridSize
		}
	}

	profile := &TerminalProfile{
		Program:    merged.program,
		Protocols:  merged.hints,
		CellSize:   merged.cellSize,
		GridSize:   merged.gridSize,
		ColorDepth: colorDepth(env),
		IsTTY:      env.TTY,
	}
	if env.TTY {
		if profile.ColorDepth >= 256 {
			addHint(&profile.Protocols, "ansiart", SupportStatic)
		}
		addHint(&profile.Protocols, "plain", SupportStatic)
	}
	return profile, nil
}

// L1: static environment variable scan.
func scanEnv(env *Environment) probeResult {
	var r probeResult
	switch env.Get("TERM_PROGRAM") {
	case "kitty":
		r.program = "kitty"
		addHint(&r.hints, "kitty", SupportNative)
	case "ghostty":
		r.program = "ghostty"
		addHint(&r.hints, "kitty", SupportNative)
	case "iTerm.app":
		r.program = "iterm2"
		addHint(&r.hints, "iterm2", SupportStatic)
	case "WezTerm":
		r.program = "wezterm"
		addHint(&r.hints, "kitty", SupportNative)
	case "Apple_Terminal":
		r.program = "apple_terminal"
	}
	if env.Get("KITTY_WINDOW_ID") != "" {
		if r.program == "" {
			r.program = "kitty"
		}
		addHint(&r.hints, "kitty", SupportNative)
	}
	if r.program == "" && env.Get("WEZTERM_PANE") != "" {
		r.program = "wezterm"
	}
	return r
}

// L2: TERM value heuristics.
func matchTerm(env *Environment) probeResult {
	var r probeResult
	switch env.Get("TERM") {
	case "xterm-kitty":
		r.program = "kitty"
		addHint(&r.hints, "kitty", SupportNative)
	case "foot", "foot-extra":
		r.program = "foot"
		addHint(&r.hints, "sixel", SupportStatic)
	case "mlterm", "yaft":
		r.program = env.Get("TERM")
		addHint(&r.hints, "sixel", SupportStatic)
	}
	return r
}

// L4: ioctl window size.
func probeSystem() probeResult {
	rows, cols, pxW, pxH, err := winSizeOf(os.Stdout.Fd())
	if err != nil || cols <= 0 {
		return probeResult{}
	}
	r := probeResult{gridSize: Size{Width: cols, Height: rows}}
	if rows > 0 && pxW > 0 {
		r.cellSize = Size{Width: pxW / cols, Height: pxH / rows}
	}
	return r
}

func addHint(hints *[]ProtocolHint, name string, level SupportLevel) {
	for i, h := range *hints {
		if h.Name == name {
			if level > h.Level {
				(*hints)[i].Level = level
			}
			return
		}
	}
	*hints = append(*hints, ProtocolHint{Name: name, Level: level})
}

func colorDepth(env *Environment) int {
	ct := strings.ToLower(env.Get("COLORTERM"))
	if ct == "truecolor" || ct == "24bit" {
		return 1 << 24
	}
	if term := env.Get("TERM"); strings.Contains(term, "256color") || strings.Contains(term, "direct") {
		return 256
	}
	return 16
}
