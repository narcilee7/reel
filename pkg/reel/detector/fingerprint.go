package detector

// hasDAParam reports whether the DA1 parameter list contains want.
func hasDAParam(params []int, want int) bool {
	for _, p := range params {
		if p == want {
			return true
		}
	}
	return false
}

// lookupDA2Fingerprint maps a DA2 response to a terminal program and its
// protocol hints, combining the fingerprint with the environment where the
// fingerprint alone is not decisive. A miss returns empty values: the raw
// parameters are recorded by the caller, nothing is guessed.
func lookupDA2Fingerprint(d2 [3]int, env *Environment) (program string, hints []ProtocolHint) {
	pv1, pv2, pv3 := d2[0], d2[1], d2[2]
	switch {
	case pv1 == 1 && pv2 >= 2000 && pv2 < 10000:
		// kitty encodes its version as e.g. 4000 for 0.40.0; animation
		// (a=f/a=a) needs >= 0.20, i.e. pv2 >= 2000.
		level := SupportNative
		if pv2 >= 2000 {
			level = SupportAnimation
		}
		return "kitty", []ProtocolHint{{Name: "kitty", Level: level}}
	case pv1 == 0 && pv2 >= 90 && pv2 <= 99:
		// iTerm2 answers DA2 with terminal type code 0.
		return "iterm2", []ProtocolHint{{Name: "iterm2", Level: SupportStatic}}
	case pv1 == 1 && pv3 == 0:
		// Generic VT220-compatible; env disambiguates wezterm, which also
		// speaks the kitty graphics protocol.
		if env != nil && env.Get("WEZTERM_PANE") != "" {
			return "wezterm", []ProtocolHint{{Name: "kitty", Level: SupportNative}}
		}
		return "", nil
	}
	return "", nil
}

// kittyLevel returns the support level for a kitty hint from static
// environment evidence: KITTY_WINDOW_ID implies a recent kitty, so animation
// support is assumed; otherwise the version is unknown and only native
// (static) support is claimed.
func kittyLevel(env *Environment) SupportLevel {
	if env.Get("KITTY_WINDOW_ID") != "" {
		return SupportAnimation
	}
	return SupportNative
}
