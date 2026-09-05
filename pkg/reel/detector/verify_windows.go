//go:build windows

package detector

// VerifyProfile is unsupported on Windows and returns the profile unchanged:
// Phase 2 detection on Windows stays at layers L1/L2, so there is nothing to
// verify.
func VerifyProfile(p *TerminalProfile) *TerminalProfile {
	return p
}
