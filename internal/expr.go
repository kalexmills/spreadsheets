package internal

import (
	"fmt"
	"strconv"
)

// ParseExpr parses the provided string into an Expr, returning an error in case of poor syntax.
func ParseExpr(str string) (Expr, error) {
	tokens, err := tokenize(str)
	if err != nil {
		return nil, err
	}
	return parseExpr(tokens)
}

// tokenize tokenizes the provided expression into a slice of []Token.
func tokenize(str string) ([]Token, error) {
	runes := []rune(str)
	if runes[0] != '=' {
		return nil, fmt.Errorf("%w: expressions must start with =", ErrExprParse)
	}
	var tokens []Token
	for i := 1; i < len(runes); i++ {
		for runes[i] == ' ' { // skip whitespace
			i++
		}
		if between(runes[i], '0', '9') {
			start := i
			for i < len(runes) && between(runes[i], '0', '9') {
				i++
			}
			tokens = append(tokens, Token(runes[start:i]))
			i--
		} else if between(runes[i], 'A', 'Z') {
			start := i
			for i < len(runes) && (between(runes[i], '0', '9') || between(runes[i], 'A', 'Z')) {
				i++
			}
			tokens = append(tokens, Token(runes[start:i]))
			i--
		} else if runes[i] == '*' {
			tokens = append(tokens, TokenMul)
		} else if runes[i] == '+' {
			tokens = append(tokens, TokenAdd)
		} else {
			return nil, fmt.Errorf("%w: unexpected character '%c'", ErrExprParse, runes[i])
		}
	}
	return tokens, nil
}

// between is true iff lb <= target && target <= ub.
func between(target rune, lb, ub rune) bool {
	return lb <= target && target <= ub
}

// parseExpr parses an expression using a recursive descent parser.
func parseExpr(tokens []Token) (Expr, error) {
	if len(tokens) == 0 {
		return nil, fmt.Errorf("%w: expected expression; found nothing", ErrExprParse)
	}
	return parseAdd(tokens)
}

func parseMul(tokens []Token) (Expr, error) {
	for i := 0; i < len(tokens)-1; i++ {
		if tokens[i] == TokenMul {
			return binExpr(i, tokens, TokenMul)
		}
	}
	return parseTerms(tokens)
}

func parseAdd(tokens []Token) (Expr, error) {
	for i := 0; i < len(tokens)-1; i++ {
		if tokens[i] == TokenAdd {
			return binExpr(i, tokens, TokenAdd)
		}
	}
	return parseMul(tokens)
}

func parseTerms(tokens []Token) (Expr, error) {
	if len(tokens) == 0 {
		return nil, fmt.Errorf("%w: expected terms; found nothing", ErrExprParse)
	}
	if cellID, err := ParseCellID(string(tokens[0])); err == nil {
		return CellRefExpr{Ref: cellID}, nil
	}
	if val, err := strconv.Atoi(string(tokens[0])); err == nil {
		return ConstExpr{Value: val}, nil
	}
	return nil, fmt.Errorf("%w: unexpected token: %s", ErrExprParse, tokens[0])
}

// binSplit splits the tokens at index i, continues parsing, and returns a BinExpr using the provided operator.
func binExpr(i int, tokens []Token, binOp Token) (Expr, error) {
	X, err := parseExpr(tokens[:i])
	if err != nil {
		return nil, err
	}
	Y, err := parseExpr(tokens[i+1:])
	if err != nil {
		return nil, err
	}
	return BinaryExpr{X: X, Op: binOp, Y: Y}, nil
}

// Expr is an interface describing an expression.
type Expr interface {
	IsExpr() // marker method, for type-safety.
}

type BinaryExpr struct {
	X  Expr  // left operand
	Op Token // operation
	Y  Expr  // right operand
}

func (b BinaryExpr) IsExpr() {}

type ConstExpr struct {
	Value int
}

func (b ConstExpr) IsExpr() {}

type CellRefExpr struct {
	Ref CellID
}

func (b CellRefExpr) IsExpr() {}

type Token string

const (
	TokenAdd Token = "+"
	TokenMul       = "*"
)

// CellRefs retrieves all cell references which are found in the expression.
func CellRefs(e Expr) []CellID {
	if e == nil {
		return nil
	}
	switch e := e.(type) {
	case BinaryExpr:
		r := CellRefs(e.Y)
		if len(r) == 0 {
			return CellRefs(e.X)
		}
		return append(CellRefs(e.X), r...)
	case ConstExpr:
		return nil
	case CellRefExpr:
		return []CellID{e.Ref}
	}
	return nil
}
