//go:build !windows

package detector

import (
	"reflect"
	"testing"
)

func TestParseDA1(t *testing.T) {
	cases := []struct {
		resp string
		want []int
	}{
		{"\x1b[?62;4;22c", []int{62, 4, 22}},
		{"\x1b[?63;1;2c", []int{63, 1, 2}},
		{"\x1b[?1;2c", []int{1, 2}},
		{"\x1b[c", nil},                  // no parameters
		{"garbage", nil},                 // not a DA response
		{"\x1b[?62;xx;4c", []int{62, 4}}, // non-numeric field skipped
	}
	for _, tc := range cases {
		if got := parseDA1([]byte(tc.resp)); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("parseDA1(%q) = %v, want %v", tc.resp, got, tc.want)
		}
	}
}

func TestParseDA2(t *testing.T) {
	cases := []struct {
		resp string
		want [3]int
	}{
		{"\x1b[>1;4000;1c", [3]int{1, 4000, 1}},
		{"\x1b[>0;95;0c", [3]int{0, 95, 0}},
		{"\x1b[>1;4c", [3]int{1, 4, -1}}, // missing third parameter
		{"garbage", [3]int{-1, -1, -1}},
	}
	for _, tc := range cases {
		if got := parseDA2([]byte(tc.resp)); got != tc.want {
			t.Errorf("parseDA2(%q) = %v, want %v", tc.resp, got, tc.want)
		}
	}
}

func TestLookupDA2Fingerprint(t *testing.T) {
	env := &Environment{Vars: map[string]string{}}
	if p, h := lookupDA2Fingerprint([3]int{1, 4000, 1}, env); p != "kitty" || len(h) != 1 || h[0].Name != "kitty" {
		t.Errorf("kitty fingerprint = %q %v", p, h)
	}
	if p, h := lookupDA2Fingerprint([3]int{0, 95, 0}, env); p != "iterm2" || len(h) != 1 || h[0].Name != "iterm2" {
		t.Errorf("iterm2 fingerprint = %q %v", p, h)
	}
	env.Vars["WEZTERM_PANE"] = "3"
	if p, h := lookupDA2Fingerprint([3]int{1, 0, 0}, env); p != "wezterm" || len(h) != 1 || h[0].Name != "kitty" {
		t.Errorf("wezterm fingerprint = %q %v", p, h)
	}
	delete(env.Vars, "WEZTERM_PANE")
	if p, _ := lookupDA2Fingerprint([3]int{1, 0, 0}, env); p != "" {
		t.Errorf("generic VT220 fingerprint = %q, want empty", p)
	}
	if p, _ := lookupDA2Fingerprint([3]int{-1, -1, -1}, env); p != "" {
		t.Errorf("absent fingerprint = %q, want empty", p)
	}
}
