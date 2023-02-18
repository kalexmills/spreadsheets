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

var runeMap = map[rune]Token{
	'+': TokenAdd,
	'-': TokenSub,
	'*': TokenMul,
	'/': TokenDiv,
}

// tokenize tokenizes the provided expression into a list of tokens, returning a ErrExprParse if any unexpected
// characters are found.
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
			// tokenize constant integer expression
			start := i
			for i < len(runes) && between(runes[i], '0', '9') {
				i++
			}
			tokens = append(tokens, Token(runes[start:i]))
			i--
		} else if between(runes[i], 'A', 'Z') {
			// tokenize cell reference
			start := i
			for i < len(runes) && (between(runes[i], '0', '9') || between(runes[i], 'A', 'Z')) {
				i++
			}
			tokens = append(tokens, Token(runes[start:i]))
			i--

		} else if token, ok := runeMap[runes[i]]; ok {
			tokens = append(tokens, token)
		} else {
			return nil, fmt.Errorf("%w: unexpected character '%c'", ErrExprParse, runes[i])
		}
	}
	return tokens, nil
}

// between is true iff target lies between lb (lower bound) and ub (upper bound).
func between(target rune, lb, ub rune) bool {
	return lb <= target && target <= ub
}

func parseExpr(tokens []Token) (Expr, error) {
	expr, rest, err := parseTerm(tokens)
	if err != nil {
		return nil, err
	}
	if len(rest) != 0 {
		return nil, fmt.Errorf("unexpected end of expression")
	}
	return expr, nil
}

func parseTerm(tokens []Token) (Expr, []Token, error) {
	var termTokens = map[Token]struct{}{TokenAdd: {}, TokenSub: {}}

	var Y Expr

	// parse out the LHS
	expr, rest, err := parseFactor2(tokens)
	if err != nil {
		return nil, nil, err
	}
	if len(rest) == 0 {
		return expr, nil, nil
	}
	// parse out as many term expressions as possible
	token := rest[0]
	_, ok := termTokens[token]
	for ok {
		Y, rest, err = parseFactor2(rest[1:])
		if err != nil {
			return nil, nil, err
		}
		expr = BinaryExpr{X: expr, Op: token, Y: Y}
		if len(rest) == 0 {
			break
		}
		token = rest[0]
		_, ok = termTokens[token]
	}
	return expr, rest, nil
}

func parseFactor2(tokens []Token) (Expr, []Token, error) {
	var factorTokens = map[Token]struct{}{TokenMul: {}, TokenDiv: {}}
	var Y Expr

	// parse out the LHS
	expr, rest, err := parseUnary2(tokens)
	if err != nil {
		return nil, nil, err
	}
	if len(rest) == 0 {
		return expr, nil, err
	}
	// continue parsing out as many factor expressions as possible
	token := rest[0]
	_, ok := factorTokens[token]
	for ok {
		Y, rest, err = parseUnary2(rest[1:])
		if err != nil {
			return nil, nil, err
		}
		expr = BinaryExpr{X: expr, Op: token, Y: Y}
		if len(rest) == 0 {
			break
		}
		token = rest[0]
		_, ok = factorTokens[token]
	}
	return expr, rest, nil
}

func parseUnary2(tokens []Token) (Expr, []Token, error) {
	if len(tokens) == 0 {
		return nil, nil, fmt.Errorf("%w: expected terms; found nothin", ErrExprParse)
	}
	if tokens[0] == TokenSub {
		X, rest, err := parseUnary2(tokens[1:])
		if err != nil {
			return nil, nil, err
		}
		return UnaryExpr{X: X, Op: TokenSub}, rest, nil
	}
	return parsePrimary2(tokens)
}

func parsePrimary2(tokens []Token) (Expr, []Token, error) {
	if len(tokens) == 0 {
		return nil, nil, fmt.Errorf("%w: expected terms; found nothing", ErrExprParse)
	}
	if cellID, err := ParseCellID(string(tokens[0])); err == nil {
		return CellRefExpr{Ref: cellID}, tokens[1:], nil
	}
	if val, err := strconv.Atoi(string(tokens[0])); err == nil {
		return ConstExpr{Value: val}, tokens[1:], nil
	}
	return nil, nil, fmt.Errorf("%w: unexpected token: %s", ErrExprParse, tokens[0])
}

// the model used here for representing parse trees is inspired by the ast package in Go's standard library.

// Expr is an interface describing an expression.
type Expr interface {
	IsExpr() // marker method, just for type-safety.
}

type UnaryExpr struct {
	X  Expr  // term
	Op Token // operation
}

// BinaryExpr represents a binary expression, containing a token representing the operation, and left and right
// operands.
type BinaryExpr struct {
	X  Expr  // left operand
	Op Token // operation
	Y  Expr  // right operand
}

// ConstExpr represents a constant valued expression.
type ConstExpr struct {
	Value int
}

// CellRefExpr represents a variable reference to another cell.
type CellRefExpr struct {
	Ref CellID
}

func (b ConstExpr) IsExpr()   {}
func (u UnaryExpr) IsExpr()   {}
func (b BinaryExpr) IsExpr()  {}
func (b CellRefExpr) IsExpr() {}

type Token string

const (
	TokenAdd Token = "+"
	TokenSub       = "-"
	TokenMul       = "*"
	TokenDiv       = "/"
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
