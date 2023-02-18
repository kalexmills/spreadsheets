package internal

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_ParseExpr(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Expr
		wantErr  bool
	}{
		{
			name:     "basic formula",
			input:    "=1+1",
			expected: add(val(1), val(1)),
		},
		{
			name:     "ignore whitespace",
			input:    "=  12 + 14",
			expected: add(val(12), val(14)),
		},
		{
			name:     "cell ref formula",
			input:    "=A1*13",
			expected: mul(cellRef(0, 0), val(13)),
		},
		{
			name:  "mul before add",
			input: "=A1*B2+C3*D4",
			expected: add(
				mul(cellRef(0, 0), cellRef(1, 1)),
				mul(cellRef(2, 2), cellRef(3, 3)),
			),
		},
		{
			name:     "complex formula",
			input:    "=123 + C4*32 + B33*5 + 354",
			expected: add(add(add(val(123), mul(cellRef(2, 3), val(32))), mul(cellRef(1, 32), val(5))), val(354)),
		},
		{
			name:     "unary expr",
			input:    "=-123",
			expected: neg(val(123)),
		},
		{
			name:     "multiply a negative",
			input:    "=-123*-456",
			expected: mul(neg(val(123)), neg(val(456))),
		},
		{
			name:     "subtract from a negative",
			input:    "=-123-456",
			expected: sub(neg(val(123)), val(456)),
		},
		{
			name:     "division",
			input:    "=A1/B2/C3/D4",
			expected: div(div(div(cellRef(0, 0), cellRef(1, 1)), cellRef(2, 2)), cellRef(3, 3)),
		},
		{
			name:    "bad expr",
			input:   "=A1*",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseExpr(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.EqualValues(t, tt.expected, parsed)
		})
	}
}

func sub(X, Y Expr) Expr {
	return BinaryExpr{X: X, Y: Y, Op: TokenSub}
}

func add(X, Y Expr) Expr {
	return BinaryExpr{X: X, Y: Y, Op: TokenAdd}
}

func mul(X, Y Expr) Expr {
	return BinaryExpr{X: X, Y: Y, Op: TokenMul}
}

func div(X, Y Expr) Expr {
	return BinaryExpr{X: X, Y: Y, Op: TokenDiv}
}

func val(x int) Expr {
	return ConstExpr{Value: x}
}

func cellRef(row, col int) Expr {
	return CellRefExpr{Ref: CellID{row: row, column: col}}
}
func neg(X Expr) Expr {
	return UnaryExpr{X: X, Op: TokenSub}
}
