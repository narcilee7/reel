// Package layout maps pixel content onto the terminal cell grid.
package layout

// CellRect is a rectangle in terminal cell coordinates.
type CellRect struct {
	Row, Col int // position; zero means the current cursor position
	Width    int
	Height   int
}
