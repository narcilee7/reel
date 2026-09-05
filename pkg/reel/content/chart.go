package content

import (
	"errors"
	"fmt"
	"math"

	"github.com/narcilee7/reel/pkg/reel/protocol"
)

// ChartType selects the chart drawing style.
type ChartType int

const (
	ChartBar ChartType = iota
	ChartLine
	// ChartScatter marks points only, without connecting lines (use
	// ChartLine for connected series).
	ChartScatter
)

// Series is one named data series; Values align one-to-one with the chart's
// Labels. YAxis selects the scale: 0 = left (default), 1 = right; dual-axis
// charts draw two independent scales, with the right ticks at the plot's
// right edge.
type Series struct {
	Name   string
	Values []float64
	YAxis  int
}

// ChartSpec is a declarative, data-only description of a chart.
type ChartSpec struct {
	Type   ChartType
	Title  string
	Labels []string // x axis categories
	Series []Series
}

// Chart renders a declarative ChartSpec as a raster image fragment, plus a
// one-line text legend for multi-series charts.
type Chart struct {
	Spec ChartSpec
}

func (c *Chart) Kind() protocol.ContentKind { return protocol.ContentKindChart }

// defaultChartCells is the render area in cells when the Context cannot
// provide one.
const (
	defaultChartCols = 60
	defaultChartRows = 25
)

// ToIR validates and rasterizes the spec to the render area (MaxCells ×
// CellSize pixels), returning an ImageFragment followed by an optional
// legend TextFragment.
func (c *Chart) ToIR(ctx protocol.Context) (*protocol.IntermediateRep, error) {
	if err := c.Spec.Validate(); err != nil {
		return nil, err
	}
	cols, rows := ctx.MaxCells.Width, ctx.MaxCells.Height
	if cols <= 0 {
		cols = defaultChartCols
	}
	if rows <= 0 {
		rows = defaultChartRows
	}
	cw, ch := ctx.CellSize.Width, ctx.CellSize.Height
	if cw <= 0 {
		cw = 10
	}
	if ch <= 0 {
		ch = 20
	}

	img, legend, err := rasterize(c.Spec, cols*cw, rows*ch)
	if err != nil {
		return nil, err
	}
	fragments := []protocol.Fragment{&protocol.ImageFragment{Image: img, Alt: c.Spec.Title}}
	if len(legend) > 0 {
		fragments = append(fragments, legend...)
	}
	return &protocol.IntermediateRep{Fragments: fragments}, nil
}

var errChartNoSeries = errors.New("content: chart needs at least one series")

// Validate rejects unusable specs before rasterization. ToIR calls it, so
// rasterize can assume consistent data (unlike Phase 2, which silently
// truncated mismatched lengths — Phase 3 turns that hidden data error into
// an explicit failure).
func (s ChartSpec) Validate() error {
	if len(s.Series) == 0 {
		return errChartNoSeries
	}
	for _, sr := range s.Series {
		if len(sr.Values) == 0 {
			return fmt.Errorf("content: series %q has no values", sr.Name)
		}
		if len(s.Labels) > 0 && len(sr.Values) != len(s.Labels) {
			return fmt.Errorf("content: series %q: %d values vs %d labels", sr.Name, len(sr.Values), len(s.Labels))
		}
		for i, v := range sr.Values {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Errorf("content: series %q: non-finite value at index %d", sr.Name, i)
			}
		}
	}
	return nil
}
