package internal

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSpreadsheet(t *testing.T) {
	t.Run("basic spreadsheet", func(t *testing.T) {
		s := NewSpreadsheet()

		assert.NoError(t, s.SetCellValue("B1", "=A1+A2+A3"))
		assert.NoError(t, s.SetCellValue("A1", 12))
		assertCellValue(t, s, "B1", 12)

		assert.NoError(t, s.SetCellValue("A2", 12))
		assertCellValue(t, s, "B1", 24)

		assert.NoError(t, s.SetCellValue("A3", 12))
		assertCellValue(t, s, "B1", 36)

		assertCellValue(t, s, "A1", 12)
		assertCellValue(t, s, "A2", 12)
		assertCellValue(t, s, "A3", 12)

		assert.NoError(t, s.SetCellValue("A2", 24))
		assertCellValue(t, s, "B1", 48)
	})

	t.Run("reference chain", func(t *testing.T) {
		s := NewSpreadsheet()

		assert.NoError(t, s.SetCellValue("A1", "=A2"))
		assert.NoError(t, s.SetCellValue("A2", "=A3"))
		assert.NoError(t, s.SetCellValue("A3", "=A4"))
		assert.NoError(t, s.SetCellValue("A4", "=A5"))
		assert.NoError(t, s.SetCellValue("A5", "=A6"))
		assert.NoError(t, s.SetCellValue("A6", "=A7"))
		assert.NoError(t, s.SetCellValue("A7", 12))

		assertCellValue(t, s, "A1", 12)
	})

	t.Run("fibonacci", func(t *testing.T) {
		s := NewSpreadsheet()

		assert.NoError(t, s.SetCellValue("A1", 0))
		assert.NoError(t, s.SetCellValue("A2", 1))
		for i := 3; i < 15; i++ {
			cell := fmt.Sprintf("A%d", i)
			expr := fmt.Sprintf("=A%d+A%d", i-2, i-1)
			assert.NoError(t, s.SetCellValue(cell, expr))
		}

		assertCellValue(t, s, "A14", 233)
	})

	t.Run("circref tiny cycle", func(t *testing.T) {
		s := NewSpreadsheet()

		assert.NoError(t, s.SetCellValue("A1", "=A2"))
		assert.ErrorIs(t, s.SetCellValue("A2", "=A1"), ErrCircRef)
	})

	t.Run("circref selfref", func(t *testing.T) {
		s := NewSpreadsheet()

		assert.ErrorIs(t, s.SetCellValue("A1", "=A1"), ErrCircRef)
	})

	t.Run("big cycle", func(t *testing.T) {
		s := NewSpreadsheet()

		for i := 1; i <= 15; i++ {
			cell1 := fmt.Sprintf("A%d", i)
			cell2 := fmt.Sprintf("=A%d", i+1)
			assert.NoError(t, s.SetCellValue(cell1, cell2))
		}
		assert.ErrorIs(t, s.SetCellValue("A15", "=A1"), ErrCircRef)
	})
}

func assertCellValue(t *testing.T, s *Spreadsheet, cellID string, expectedValue int) {
	t.Helper()
	val, err := s.GetCellValue(cellID)
	assert.NoError(t, err)
	assert.EqualValues(t, expectedValue, val)
}

func TestSpreadsheet_eval(t *testing.T) {
	tests := []struct {
		name     string
		sheet    *Spreadsheet
		expr     Expr
		expected int
	}{
		{
			name:  "basic",
			sheet: spreadsheet([][]any{{1, 2}, {3, 4}}),
			expr: add(
				add(cellRef(0, 0), cellRef(0, 1)),
				add(cellRef(1, 0), cellRef(1, 1)),
			),
			expected: 10,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sheet.evalExpr(tt.expr)
			assert.EqualValues(t, tt.expected, got)
		})
	}
}

func spreadsheet(input [][]any) *Spreadsheet {
	result := NewSpreadsheet()
	for r := 0; r < len(input); r++ {
		for c := 0; c < len(input[r]); c++ {
			cid := CellID{row: r, column: c}
			switch val := input[r][c].(type) {
			case int:
				result.cells[cid] = &Cell{currValue: val}
			case Expr:
				result.cells[cid] = &Cell{expr: &val}
			}
		}
	}
	return result
}

func Test_ParseCellID(t *testing.T) {
	tests := map[string]CellID{
		"A1":   {row: 0, column: 0},
		"AB32": {row: 27, column: 31},
		"Z25":  {row: 25, column: 24},
	}
	for in, want := range tests {
		got, err := ParseCellID(in)
		assert.NoError(t, err)
		assert.Equal(t, want, got)
	}
}

func Test_decodeRowExpr(t *testing.T) {
	tests := map[string]int{
		"A":   0,
		"Z":   25,
		"AA":  26,
		"AB":  27,
		"AZ":  51,
		"FS":  6*26 + 18,
		"ABC": 1*26*26 + 2*26 + 2,
	}
	for in, want := range tests {
		got, err := decodeRowExpr(in)
		assert.NoError(t, err)
		assert.Equal(t, want, got)
	}
}
