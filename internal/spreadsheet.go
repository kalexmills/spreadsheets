package internal

import (
	"errors"
	"fmt"
	"golang.org/x/exp/maps"
	"regexp"
	"strconv"
)

var (
	ErrParseCellID = errors.New("could not parse input as a valid CellID")
	ErrValueType   = errors.New("unexpected type for cell value")
	ErrExprParse   = errors.New("parse error")
)

// Spreadsheet represents a spreadsheet capable of setting and retrieving cell values. Cells in this spreadsheet store
// integers. All cells start with a value of 0. Each cell contains either a raw value, or an expression in the format
//
//	=A1+B2*C3
//
// Only addition and multiplication are supported as binary operations.
type Spreadsheet struct {
	// cells maps from CellID to cells.
	cells map[CellID]*Cell
	// refersTo maps cells to all cells directly referenced in its current expression. It is the inverse of referedFrom.
	refersTo map[CellID]map[CellID]struct{}
	// referedFrom maps cells to the set of all cells that reference them. It is the inverse of refersTo.
	referedFrom map[CellID]map[CellID]struct{}
}

type Cell struct {
	currValue int   // currValue is the current value of this cell
	expr      *Expr // expr describes the expression used to compute
}

// SetCellValue sets the value of the cell with the provided cell ID. Val can be either an int or a valid string
// expression which the cell ought to contain. An error is returned if the expression cannot be parsed, an invalid
// cellID is provided, or val is some type other than int or string.
func (s *Spreadsheet) SetCellValue(cellID string, val any) error {
	cid, err := ParseCellID(cellID)
	if err != nil {
		return err
	}
	switch val := val.(type) {
	case int:
		s.cells[cid].expr = nil      // unset expr
		s.cells[cid].currValue = val // set value
	case string:
		expr, err := ParseExpr(val)
		if err != nil {
			return err
		}
		s.cells[cid].expr = &expr  // set expr
		s.cells[cid].currValue = 0 // unset value
		s.refresh(cid)
	default:
		return fmt.Errorf("%w: only int and string are allowed", ErrValueType)
	}
	return nil
}

// GetCellValue retrieves the value of the cell with the provided ID. An error is returned if the provided string could
// not be parsed as a valid cell ID.
func (s *Spreadsheet) GetCellValue(cellID string) (int, error) {
	cid, err := ParseCellID(cellID)
	if err != nil {
		return 0, err
	}
	cell, ok := s.cells[cid]
	if !ok {
		return 0, nil // empty cells always have a value of zero.
	}
	return cell.currValue, nil // all cell values are pre-computed by SetCellValue
}

// eval evaluates the provided expression. results reported by eval are only valid when called in topological order
// during refresh. It does not track circular references on its own.
func (s *Spreadsheet) eval(expr Expr) int {
	switch expr := expr.(type) {
	case BinaryExpr:
		x := s.eval(expr.X)
		y := s.eval(expr.Y)
		switch expr.Op {
		case TokenAdd:
			return x + y
		case TokenMul:
			return x * y
		}
	case ConstExpr:
		return expr.Value
	case CellRefExpr:
		cell := s.cells[expr.Ref]
		return cell.currValue
	}
	return 0 // "unreachable" if parseExpr is valid
}

// refresh refreshes the spreadsheet, with the knowledge that cell cid was just updated.
func (s *Spreadsheet) refresh(cid CellID) {
	// update refersTo with new information (if needed)
	cell, ok := s.cells[cid]
	if !ok {
		return
	}
	if cell.expr != nil {
		// unset referedFrom refs and clear out refersTo refs.
		for ref := range s.refersTo[cid] {
			delete(s.referedFrom[ref], cid)
		}
		maps.Clear(s.refersTo[cid])
		// update the graph with new refs
		for _, ref := range CellRefs(*cell.expr) {
			s.refersTo[cid][ref] = struct{}{}
			s.referedFrom[ref][cid] = struct{}{}
		}
	}
	// get the ancestors, check for a cycle

	// topological sort to traverse, check for cycles
}

// C3 =A1+B2
//
// A1->C3
// B2->C3

// greatestAncestors retrieves the greatest ancestors of the provided cell: cells that don't reference other cells.
func (s Spreadsheet) greatestAncestors(cid CellID) []CellID {
	// BFS in
	return nil
}

// CellID represents a column and row of our spreadsheet.
type CellID struct {
	row    int // row, zero-indexed
	column int // column, zero-indexed.
}

// NewCellID forms a new CellID from the provided row and column, both of which are zero-indexed.
func NewCellID(column, row uint) CellID {
	return CellID{column: int(column), row: int(row)}
}

// note: Golang's regular expressions are guaranteed to run in linear time in the size of their input.
var cellRegexp = regexp.MustCompile("([A-Z]+)([0-9]+)")

// ParseCellID parses the provided string as a CellID, returning an error if it is unable to do so.
// As strings, CellIDs are 1-indexed; this func converts to zero-indexed CellID as expected.
func ParseCellID(str string) (CellID, error) {
	groups := cellRegexp.FindStringSubmatch(str)
	if len(groups) != 3 {
		return CellID{}, ErrParseCellID
	}
	rowExpr, colExpr := groups[1], groups[2]
	rowIdx, err := decodeRowExpr(rowExpr)
	if err != nil {
		return CellID{}, fmt.Errorf("%w: unexpected row format '%s'", ErrParseCellID, str)
	}
	colIdx, err := strconv.Atoi(colExpr)
	if err != nil {
		return CellID{}, fmt.Errorf("%w: unexpected column format '%s'", ErrParseCellID, str)
	}
	return CellID{row: rowIdx, column: colIdx - 1}, nil
}

// decodeRowExpr decodes a 'base-26' row expression into its equivalent integer, returning an error if it is unable to
// do so.
func decodeRowExpr(str string) (int, error) {
	// TODO: check for 64-bit integer overflow; note that catching a panic doesn't work in production since
	//       detecting integer overflow/underflow is disabled in production Go code.
	var runes []rune
	for _, ch := range str {
		if ch < 'A' || 'Z' < ch {
			return 0, ErrParseCellID
		}
		runes = append(runes, ch)
	}

	for i := 0; i < len(runes)/2; i++ { // reverse runes
		runes[i], runes[len(runes)-1-i] = runes[len(runes)-1-i], runes[i]
	}

	const base = 26
	currBase := 1
	sum := 0
	for i := 0; i < len(runes); i++ {
		digit := int(runes[i] - 'A' + 1)
		sum += digit * currBase
		currBase *= base
	}
	return sum - 1, nil
}

// String returns a string representation of this cellID.
func (c CellID) String() string {
	return "" // TODO: this; for debugging
}

// ColIdx provides the 0-based row index for this cell.
func (c CellID) ColIdx() int {
	return c.column
}

// RowIdx provides the 0-based row index for this cell.
func (c CellID) RowIdx() int {
	return c.row
}
