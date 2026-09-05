package reel

import (
	"os"
	"strconv"

	"github.com/narcilee7/reel/pkg/reel/detector"
)

// Diagnostic is the raw evidence plus conclusion of one full probe round,
// for debugging and DA2 fingerprint calibration. ProbeDiagnostic bypasses
// the cross-process cache and always attempts the L3 DA query on a TTY, so
// every field reflects this process's own observations.
type Diagnostic struct {
	// Env is the subset of environment variables that participates in
	// detection.
	Env map[string]string `json:"env"`
	// DA1Raw/DA2Raw are the escaped raw DA responses (Go string escaping);
	// empty means the query was not sent or the response never arrived.
	DA1Raw string `json:"da1_raw"`
	DA2Raw string `json:"da2_raw"`
	// DA1Params/DA2 are the parsed responses; DA2 entries are -1 when
	// absent. Both are empty/zero when the raw fields are empty.
	DA1Params []int  `json:"da1_params"`
	DA2       [3]int `json:"da2"`
	// Profile is the final detection conclusion.
	Profile *TerminalProfile `json:"profile"`
	// CacheHit is always false for ProbeDiagnostic (the cache is bypassed);
	// the field exists so JSON consumers see the distinction.
	CacheHit bool `json:"cache_hit"`
	// Resolution records where the program conclusion came from:
	// "da2", "env", "term" or "unknown".
	Resolution string `json:"resolution"`
}

// envKeys lists the environment variables the detector consults.
var envKeys = []string{
	"TERM", "TERM_PROGRAM", "COLORTERM",
	"KITTY_WINDOW_ID", "WEZTERM_PANE", "TMUX",
}

// ProbeDiagnostic forces a fresh, cache-bypassing probe (including the L3
// DA query on TTYs) and returns the raw evidence. Non-TTY environments and
// failed DA rounds leave the corresponding fields empty — the probe shows
// what happened; it does not demand success.
func ProbeDiagnostic() (*Diagnostic, error) {
	d := detector.NewDetector(detector.EnvironmentFromOS())
	profile, da, resolution, err := d.ProbeRaw()
	if err != nil {
		return nil, err
	}
	diag := &Diagnostic{
		Env:        make(map[string]string, len(envKeys)),
		Profile:    profile,
		Resolution: resolution,
	}
	for _, k := range envKeys {
		if v, ok := os.LookupEnv(k); ok {
			diag.Env[k] = v
		}
	}
	if da != nil {
		if da.Raw1 != nil {
			diag.DA1Raw = strconv.Quote(string(da.Raw1))
			diag.DA1Params = da.DA1Params
		}
		if da.Raw2 != nil {
			diag.DA2Raw = strconv.Quote(string(da.Raw2))
			diag.DA2 = da.DA2
		}
	}
	return diag, nil
}
