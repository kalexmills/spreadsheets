package internal

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

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
