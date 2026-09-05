// Package detector probes the terminal emulator and caches a TerminalProfile,
// so protocol selection at render time is a plain table lookup.
package detector

import (
	"os"
	"strings"
	"time"
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

// Probe runs the detection layers: L1 environment variables, L2 TERM
// heuristics and L4 ioctl window size at zero cost, plus L3 DA1/DA2
// arbitration when L1 and L2 conflict (or both are empty on a TTY). Results
// are served from and stored to the cross-process cache when possible.
func (d *Detector) Probe() (*TerminalProfile, error) {
	env := d.env
	if entry := loadCache(env); entry != nil {
		return entry.Profile, nil
	}

	l1 := scanEnv(env)
	l2 := matchTerm(env)
	sys := probeSystem()

	merged := probeResult{}
	for _, r := range []probeResult{l1, l2, sys} {
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

	viaDA := false
	if env.TTY && needsArbitration(l1.program, l2.program) {
		if da, err := queryDeviceAttributes(daTimeout); err == nil && da != nil {
			viaDA = true
			program, hints := lookupDA2Fingerprint(da.DA2, env)
			if program != "" {
				merged.program = program
			}
			for _, h := range hints {
				addHint(&merged.hints, h.Name, h.Level)
			}
			if hasDAParam(da.DA1Params, 4) {
				addHint(&merged.hints, "sixel", SupportStatic)
			}
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
		storeCache(env, profile, viaDA)
	}
	return profile, nil
}

// ProbeRaw forces a fresh probe, bypassing the cross-process cache, and runs
// the L3 DA round whenever a TTY is available — even when L1 and L2 agree —
// so diagnostics can see the raw responses. The returned DAResult is nil
// when the query was impossible or failed; that is not an error. The
// resolution string records where the program conclusion came from: "da2",
// "env", "term", or "unknown".
//
// Unlike Probe, ProbeRaw never writes the cache.
func (d *Detector) ProbeRaw() (*TerminalProfile, *DAResult, string, error) {
	env := d.env

	l1 := scanEnv(env)
	l2 := matchTerm(env)
	sys := probeSystem()

	merged := probeResult{}
	for _, r := range []probeResult{l1, l2, sys} {
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

	resolution := "unknown"
	var daRes *DAResult
	if env.TTY {
		if da, err := queryDeviceAttributes(daTimeout); err == nil && da != nil {
			daRes = da
			program, hints := lookupDA2Fingerprint(da.DA2, env)
			if program != "" {
				resolution = "da2"
				merged.program = program
			}
			for _, h := range hints {
				addHint(&merged.hints, h.Name, h.Level)
			}
			if hasDAParam(da.DA1Params, 4) {
				addHint(&merged.hints, "sixel", SupportStatic)
			}
		}
	}
	if resolution == "unknown" {
		switch {
		case l1.program != "":
			resolution = "env"
		case l2.program != "":
			resolution = "term"
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
	return profile, daRes, resolution, nil
}

// L1: static environment variable scan.
func scanEnv(env *Environment) probeResult {
	var r probeResult
	switch env.Get("TERM_PROGRAM") {
	case "kitty":
		r.program = "kitty"
		addHint(&r.hints, "kitty", kittyLevel(env))
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
		addHint(&r.hints, "kitty", kittyLevel(env))
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

// daTimeout bounds the DA1/DA2 round trip during L3 arbitration.
const daTimeout = 500 * time.Millisecond

// needsArbitration reports whether L3 DA queries should run: when the L1 and
// L2 program conclusions conflict, or when both are empty on a TTY (e.g. SSH
// or tmux with a scrubbed environment).
func needsArbitration(l1, l2 string) bool {
	if l1 != "" && l2 != "" {
		return l1 != l2
	}
	return l1 == "" && l2 == ""
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
