package content

import (
	"math"
	"strings"
	"testing"

	"github.com/narcilee7/reel/pkg/reel/protocol"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		spec    ChartSpec
		wantErr string
	}{
		{"ok", ChartSpec{Series: []Series{{Values: []float64{1, 2}}}}, ""},
		{"no series", ChartSpec{}, "needs at least one series"},
		{"empty values", ChartSpec{Series: []Series{{Name: "s", Values: nil}}}, `series "s" has no values`},
		{"len mismatch", ChartSpec{Labels: []string{"a", "b"}, Series: []Series{{Name: "s", Values: []float64{1}}}}, `series "s": 1 values vs 2 labels`},
		{"nan", ChartSpec{Series: []Series{{Name: "s", Values: []float64{1, math.NaN()}}}}, `series "s": non-finite value at index 1`},
		{"inf", ChartSpec{Series: []Series{{Name: "s", Values: []float64{math.Inf(-1)}}}}, "non-finite"},
	}
	for _, tc := range cases {
		err := tc.spec.Validate()
		if tc.wantErr == "" {
			if err != nil {
				t.Errorf("%s: %v", tc.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("%s: err = %v, want %q", tc.name, err, tc.wantErr)
		}
	}
}

func TestNiceStep(t *testing.T) {
	cases := []struct {
		raw  float64
		want float64
	}{
		{0.3, 0.5},
		{2.1, 5},
		{1, 1},
		{0.04, 0.05},
		{10, 10},
		{11, 20},
		{100, 100},
	}
	for _, tc := range cases {
		if got := niceStep(tc.raw); got != tc.want {
			t.Errorf("niceStep(%v) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

func TestAxisRangeDualIndependent(t *testing.T) {
	spec := ChartSpec{
		Type: ChartLine,
		Series: []Series{
			{Name: "left", Values: []float64{0, 10}},
			{Name: "right", Values: []float64{1000, 2000}, YAxis: 1},
		},
	}
	loMin, loMax, _ := axisRange(spec, 0, 400)
	hiMin, hiMax, _ := axisRange(spec, 1, 400)
	if loMax > 20 || loMin > 2 {
		t.Errorf("left axis range = [%v, %v], want around [0, 10]", loMin, loMax)
	}
	if hiMin > 1000 || hiMax < 2050 {
		t.Errorf("right axis range = [%v, %v], want to cover [1000, 2000]", hiMin, hiMax)
	}
}

func TestAxisRangeBarIncludesZero(t *testing.T) {
	spec := ChartSpec{Type: ChartBar, Series: []Series{{Values: []float64{3, 7}}}}
	ymin, _, _ := axisRange(spec, 0, 400)
	if ymin > 0 {
		t.Errorf("bar axis ymin = %v, want <= 0", ymin)
	}
}

func TestScatterPixels(t *testing.T) {
	spec := ChartSpec{
		Type:   ChartScatter,
		Labels: []string{"a", "b", "c"},
		Series: []Series{{Name: "s", Values: []float64{1, 2, 3}}},
	}
	img, _, err := rasterize(spec, 600, 400)
	if err != nil {
		t.Fatal(err)
	}
	want := seriesColor(0)
	found := 0
	for y := 0; y < 400; y++ {
		for x := 0; x < 600; x++ {
			if img.RGBAAt(x, y) == want {
				found++
			}
		}
	}
	if found < 3*9 { // three 3×3 points (may merge, but never fewer than one block per point... assert generously)
		t.Errorf("scatter pixels = %d, want many", found)
	}
}

func TestLegendConditions(t *testing.T) {
	// Multi-series: legend.
	spec := ChartSpec{Series: []Series{{Values: []float64{1}}, {Values: []float64{2}}}}
	if legendFragments(spec) == nil {
		t.Error("multi-series chart must have a legend")
	}
	// Single named series: legend (color anchor).
	spec = ChartSpec{Series: []Series{{Name: "x", Values: []float64{1}}}}
	leg := legendFragments(spec)
	if leg == nil || !strings.Contains(leg[0].(*protocol.TextFragment).Text, "■ x") {
		t.Errorf("single named series must have a legend: %v", leg)
	}
	// Single unnamed series: no legend.
	spec = ChartSpec{Series: []Series{{Values: []float64{1}}}}
	if legendFragments(spec) != nil {
		t.Error("single unnamed series must not have a legend")
	}
}

func TestToIRRejectsInvalid(t *testing.T) {
	c := &Chart{Spec: ChartSpec{Labels: []string{"a", "b"}, Series: []Series{{Values: []float64{1}}}}}
	if _, err := c.ToIR(protocol.Context{}); err == nil {
		t.Error("ToIR must reject invalid specs")
	}
}
