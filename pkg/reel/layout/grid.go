package layout

// Grid describes the terminal cell geometry.
type Grid struct {
	CellWidth  int // cell width in pixels
	CellHeight int // cell height in pixels
	Cols       int
	Rows       int
}

// NewGrid returns a Grid with the given cell pixel size and dimensions.
func NewGrid(cellWidth, cellHeight, cols, rows int) *Grid {
	return &Grid{
		CellWidth:  cellWidth,
		CellHeight: cellHeight,
		Cols:       cols,
		Rows:       rows,
	}
}
