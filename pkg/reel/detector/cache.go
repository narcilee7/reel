package detector

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Cache TTLs: statically merged profiles are stable for a long time, while
// profiles arbitrated via DA carry terminal-version specifics and expire
// sooner.
const (
	cacheTTLStatic = 7 * 24 * time.Hour
	cacheTTLDA     = 24 * time.Hour
)

// cacheEntry is one record of the cross-process probe cache.
type cacheEntry struct {
	Key      string           // cacheKey() hash of the environment
	Profile  *TerminalProfile `json:"profile"`
	ProbedAt time.Time        `json:"probed_at"`
	ViaDA    bool             `json:"via_da"` // true = arbitrated by DA2, shorter TTL
}

// cachePath returns ${XDG_CACHE_HOME:-~/.cache}/reel/profile.json.
func cachePath() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "reel", "profile.json")
}

// cacheEnabled reports whether the cross-process cache may be used.
// REEL_CACHE=0 disables both reads and writes.
func cacheEnabled() bool {
	return os.Getenv("REEL_CACHE") != "0"
}

// cacheKey hashes the environment fields that determine a profile; any
// change invalidates the cached entry.
func cacheKey(env *Environment) string {
	h := sha1.New()
	for _, k := range []string{"TERM_PROGRAM", "TERM_PROGRAM_VERSION", "TERM", "COLORTERM"} {
		fmt.Fprintf(h, "%s=%s\n", k, env.Get(k))
	}
	if env.Get("KITTY_WINDOW_ID") != "" {
		h.Write([]byte("KITTY_WINDOW_ID\n"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// loadCache returns a fresh cache entry for the environment, or nil when
// caching is disabled, the terminal is not a TTY, or the entry is missing,
// stale or keyed differently. Cache failures never affect probing.
func loadCache(env *Environment) *cacheEntry {
	if !cacheEnabled() || !env.TTY {
		return nil
	}
	path := cachePath()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var e cacheEntry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil
	}
	if e.Profile == nil || e.Key != cacheKey(env) {
		return nil
	}
	ttl := cacheTTLStatic
	if e.ViaDA {
		ttl = cacheTTLDA
	}
	if time.Since(e.ProbedAt) > ttl {
		return nil
	}
	return &e
}

// storeCache writes the profile to the cache, failing silently: an
// unwritable cache directory must not affect rendering.
func storeCache(env *Environment, p *TerminalProfile, viaDA bool) {
	if !cacheEnabled() || !env.TTY || p == nil {
		return
	}
	path := cachePath()
	if path == "" {
		return
	}
	e := cacheEntry{Key: cacheKey(env), Profile: p, ProbedAt: time.Now(), ViaDA: viaDA}
	data, err := json.Marshal(&e)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}
