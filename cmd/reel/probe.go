package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/narcilee7/reel/pkg/reel"
)

// runProbe implements `reel probe [-json|-template]`: terminal detection
// diagnostics for fingerprint calibration. All evidence comes from the root
// package's public Diagnostic API; this file only formats it.
func runProbe(args []string) error {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "machine-readable JSON output")
	asTemplate := fs.Bool("template", false, "markdown snippet for a GitHub issue")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: reel probe [-json|-template]")
	}
	if *asJSON && *asTemplate {
		return fmt.Errorf("reel probe: -json and -template are mutually exclusive")
	}

	diag, err := reel.ProbeDiagnostic()
	if err != nil {
		return err
	}

	switch {
	case *asJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(diag)
	case *asTemplate:
		return printProbeTemplate(os.Stdout, diag)
	default:
		return printProbeHuman(os.Stdout, diag)
	}
}

func printProbeHuman(w io.Writer, d *reel.Diagnostic) error {
	var b strings.Builder
	b.WriteString("reel probe — terminal detection diagnostics\n\nEnvironment:\n")
	keys := make([]string, 0, len(d.Env))
	for k := range d.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "  %s = %s\n", k, d.Env[k])
	}

	b.WriteString("\nDevice attributes (DA):\n")
	if d.DA1Raw == "" && d.DA2Raw == "" {
		b.WriteString("  (no DA response — not a TTY, or the terminal did not answer)\n")
	} else {
		if d.DA1Raw != "" {
			fmt.Fprintf(&b, "  DA1 raw:    %s\n  DA1 params: %v\n", d.DA1Raw, d.DA1Params)
		}
		if d.DA2Raw != "" {
			fmt.Fprintf(&b, "  DA2 raw:    %s\n  DA2 parsed: %v\n", d.DA2Raw, d.DA2)
		}
	}

	p := d.Profile
	fmt.Fprintf(&b, "\nProfile:\n  program:     %s\n  cell size:   %dx%d px\n  grid:        %dx%d\n  color depth: %d\n  TTY:         %v\n",
		p.Program, p.CellSize.Width, p.CellSize.Height, p.GridSize.Width, p.GridSize.Height, p.ColorDepth, p.IsTTY)

	b.WriteString("\nProtocol hints:\n")
	if len(p.Protocols) == 0 {
		b.WriteString("  (none)\n")
	}
	for _, h := range p.Protocols {
		fmt.Fprintf(&b, "  %s: %s\n", h.Name, h.Level)
	}
	fmt.Fprintf(&b, "\nResolution: %s (cache hit: %v)\n", d.Resolution, d.CacheHit)
	_, err := io.WriteString(w, b.String())
	return err
}

// printProbeTemplate renders a ready-to-paste GitHub issue section: the
// probe output in a fenced block, with placeholders for the reporter to
// fill in the terminal name and version.
func printProbeTemplate(w io.Writer, d *reel.Diagnostic) error {
	var body strings.Builder
	printProbeHuman(&body, d)

	var b strings.Builder
	b.WriteString("### Environment\n\n")
	b.WriteString("- Terminal: <!-- name and version, e.g. kitty 0.44.0 -->\n")
	b.WriteString("- Platform: <!-- macOS / Linux / Windows -->\n")
	b.WriteString("- reel version: <!-- output of `reel probe` resolution line, or commit -->\n\n")
	b.WriteString("<details><summary><code>reel probe</code> output</summary>\n\n")
	b.WriteString("```\n$ reel probe\n")
	b.WriteString(body.String())
	b.WriteString("```\n\n</details>\n")
	_, err := io.WriteString(w, b.String())
	return err
}
