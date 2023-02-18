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
// integers. All cells start with a value of 0. Each cell contains either a raw integer value, or an expression in the
// format
//
//	=A1+B2*C3+12
//
// Only addition and multiplication are supported as binary operations.
type Spreadsheet struct {
	// cells maps from CellID to cells.
	cells map[CellID]*Cell
	// refersTo maps cells to all cells directly referenced by its current expression. It is the inverse of referredFrom.
	refersTo map[CellID]map[CellID]struct{}
	// referredFrom maps cells to the set of all cells that directly reference them. It is the inverse of refersTo.
	referredFrom map[CellID]map[CellID]struct{}
}

func NewSpreadsheet() *Spreadsheet {
	return &Spreadsheet{
		cells:        make(map[CellID]*Cell),
		refersTo:     make(map[CellID]map[CellID]struct{}),
		referredFrom: make(map[CellID]map[CellID]struct{}),
	}
}

// Cell represents a single cell of a spreadsheet.
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
	if _, ok := s.cells[cid]; !ok {
		s.cells[cid] = &Cell{}
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
	default:
		return fmt.Errorf("%w: only int and string are allowed", ErrValueType)
	}
	return s.refresh(cid)
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
	return cell.currValue, nil // all cell values are pre-computed by SetCellValue during refresh.
}

// eval evaluates the value of the provided cell.
func (s *Spreadsheet) eval(cid CellID) int {
	cell, ok := s.cells[cid]
	if !ok {
		return 0 // all missing cells have a value of 0.
	}
	if cell.expr == nil {
		return cell.currValue
	}
	return s.evalExpr(*cell.expr)
}

// evalExpr evaluates the provided expression. results reported by evalExpr are only valid when cells are called in
// topological order during refresh. evalExpr does not track circular references on its own.
func (s *Spreadsheet) evalExpr(expr Expr) int {
	switch expr := expr.(type) {
	case UnaryExpr:
		if expr.Op == TokenSub {
			x := s.evalExpr(expr.X)
			return -x
		}
	case BinaryExpr:
		x := s.evalExpr(expr.X)
		y := s.evalExpr(expr.Y)
		switch expr.Op {
		case TokenAdd:
			return x + y
		case TokenMul:
			return x * y
		case TokenSub:
			return x - y
		case TokenDiv:
			if y == 0 {
				return 0 // refuse to divide by zero TODO: alert the user; like a circ ref
			}
			return x / y
		}
	case ConstExpr:
		return expr.Value
	case CellRefExpr:
		if cell, ok := s.cells[expr.Ref]; ok {
			return cell.currValue
		}
		return 0 // empty cells are zeroes.
	}
	return 0 // "unreachable" if parseExpr is valid
}

// refresh refreshes the spreadsheet, with the knowledge that cell cid was just updated.
func (s *Spreadsheet) refresh(cid CellID) error {
	cell, ok := s.cells[cid]
	if !ok {
		return nil // nothing to see here
	}

	// update refersTo and referredFrom (if needed)
	if cell.expr != nil {
		// unset referredFrom refs and clear out refersTo refs.
		for ref := range s.refersTo[cid] {
			delete(s.referredFrom[ref], cid)
		}
		maps.Clear(s.refersTo[cid])
		// update the graph with new refs
		for _, ref := range CellRefs(*cell.expr) {
			s.addCellReferral(cid, ref)
		}
	}

	// get start nodes; these are the cells transitively referring to cid which are not referred to by anyone else.
	// They will form the start point of the topological sort we're about to do to ensure that we re-evaluate cells in
	// the correct order.
	roots := s.rootReferrers(cid)

	// Topological sort to re-evaluate cells in the correct order & check for circular references at the same time.
	order, err := s.topSort(roots)
	if err != nil {
		return err // circular reference detected; bail!
		// FIXME: be more user-friendly like Excel and allow circular references to exist without throwing an error.
	}

	// re-evaluate all the cells found in topological order.
	for _, cid := range order {
		if cell, ok := s.cells[cid]; ok {
			cell.currValue = s.eval(cid)
		}
	}
	return nil
}

// addCellReferral adds edges to the graph so that source refers to target.
func (s *Spreadsheet) addCellReferral(source, target CellID) {
	if _, ok := s.refersTo[source]; !ok {
		s.refersTo[source] = make(map[CellID]struct{})
	}
	if _, ok := s.referredFrom[target]; !ok {
		s.referredFrom[target] = make(map[CellID]struct{})
	}

	s.refersTo[source][target] = struct{}{}
	s.referredFrom[target][source] = struct{}{}
}

// rootReferrers retrieves all unreferenced cells which transitively refer to cid.
func (s *Spreadsheet) rootReferrers(cid CellID) []CellID {
	// BFS from cid over all ancestors to find starting cells
	frontier := []CellID{cid}
	seen := map[CellID]struct{}{cid: {}}
	var startCells []CellID
	for len(frontier) > 0 {
		curr := frontier[0]
		frontier = frontier[1:]
		if referrers, ok := s.referredFrom[curr]; !ok || len(referrers) == 0 {
			startCells = append(startCells, curr)
		}

		for referer := range s.referredFrom[curr] {
			if _, sawReferer := seen[referer]; !sawReferer {
				frontier = append(frontier, referer)
				seen[referer] = struct{}{}
			}
		}
	}
	if len(startCells) == 0 {
		return []CellID{cid}
	}
	return startCells
}

// ErrCircRef is returned whenever a circular reference is added.
var ErrCircRef = errors.New("circular reference detected")

// topSort implements a topological sort. Only nodes reachable from the provided startNodes will be sorted and included
// in the output.
func (s *Spreadsheet) topSort(startNodes []CellID) ([]CellID, error) {
	var result []CellID

	perm := make(map[CellID]struct{})
	temp := make(map[CellID]struct{})

	// recursive DFS to perform a topological sort without destroying the graph structure.
	var visit func(curr CellID) error
	visit = func(curr CellID) error {
		if _, permMark := perm[curr]; permMark {
			return nil
		}
		if _, tempMark := temp[curr]; tempMark {
			return ErrCircRef
		}
		temp[curr] = struct{}{}

		for neighbor := range s.refersTo[curr] {
			if err := visit(neighbor); err != nil {
				return err
			}
		}
		delete(temp, curr)
		perm[curr] = struct{}{}
		result = append(result, curr)
		return nil
	}

	// visit each of the starting nodes
	for _, node := range startNodes {
		if err := visit(node); err != nil {
			return nil, err
		}
	}
	return result, nil
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
//
// This func does not currently check for integer overflow.
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

// ColIdx provides the 0-based row index for this cell.
func (c CellID) ColIdx() int {
	return c.column
}

// RowIdx provides the 0-based row index for this cell.
func (c CellID) RowIdx() int {
	return c.row
}
