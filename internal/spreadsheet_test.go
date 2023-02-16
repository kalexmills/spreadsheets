package internal

import (
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
