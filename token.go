package lexer

import (
	"context"
	"fmt"
	"math"
	"unicode"
)

// TokenType represents the type of the token
type TokenType uint32

const (
	_ TokenType = iota

	// TokenTypeValue represents a generic token value. This can be real numbers or other types.
	TokenTypeValue

	// TokenTypeComment represents a comment token
	TokenTypeComment

	// TokenTypeKeyword represents a keyword like `let` or `const`
	TokenTypeKeyword

	// TokenTypeOperator represents an operator e.g. `=`
	TokenTypeOperator

	// TokenTypeString represents a string type. The registered run is the the open & close values.
	TokenTypeString

	// TokenTypeTerminator represents
	TokenTypeTerminator

	// TokenTypeNewLine represents a newline
	TokenTypeNewLine

	// TokenTypeSyntax represents a syntax type value, like open or close parenthesis
	TokenTypeSyntax

	_testTokenTypeMaxValue
)

// Token represents a single code point
type Token struct {
	Type     TokenType
	Value    string
	Position Position
}

// Accept processes the visitor and calls the appropriate visitor function
func (t Token) Accept(ctx context.Context, v Visitor) error {
	switch t.Type {
	case TokenTypeValue:
		return v.VisitValue(ctx, t)
	case TokenTypeComment:
		return v.VisitComment(ctx, t)
	case TokenTypeKeyword:
		return v.VisitKeyword(ctx, t)
	case TokenTypeOperator:
		return v.VisitOperator(ctx, t)
	case TokenTypeString:
		return v.VisitString(ctx, t)
	case TokenTypeTerminator:
		return v.VisitTerminator(ctx, t)
	case TokenTypeNewLine:
		return v.VisitNewLine(ctx, t)
	case TokenTypeSyntax:
		return v.VisitSyntax(ctx, t)
	default:
		panic(fmt.Sprintf("unexpected token type: %s", t.Type))
	}
}

// Position represents where the token was found
type Position struct {
	ID    string
	Range Range
}

// Range is a character range
type Range struct {
	Start Location
	End   Location
}

// Location is a specific location of the source
type Location struct {
	Line   uint32
	Column uint32
}

func (l Location) String() string {
	return fmt.Sprintf("(%d, %d)", l.Line, l.Column)
}

const (
	// LocationValueInvalid represents an invalid location value for either line
	// or column
	LocationValueInvalid = math.MaxUint32
)

func mLoc(l, c uint32) Location {
	return Location{Line: l, Column: c}
}

func mRange(sl, el, sc, ec uint32) Range {
	return Range{
		Start: mLoc(sl, sc),
		End:   mLoc(el, ec),
	}
}

func mPosition(id string, sl, el, sc, ec uint32) Position {
	return Position{
		ID:    id,
		Range: mRange(sl, el, sc, ec),
	}
}

func (t TokenType) String() string {
	switch t {
	case TokenTypeValue:
		return "VAL"
	case TokenTypeComment:
		return "COM"
	case TokenTypeKeyword:
		return "KEY"
	case TokenTypeOperator:
		return "OPR"
	case TokenTypeString:
		return "STR"
	case TokenTypeTerminator:
		return "TER"
	case TokenTypeNewLine:
		return "NWL"
	case TokenTypeSyntax:
		return "SNT"
	default:
		return fmt.Sprintf("UNKNOWN_%d", t)
	}
}

// IsNumeric returns true if all the runes in the token value are numbers
func (t Token) IsNumeric() bool {
	for _, v := range t.Value {
		if !unicode.IsDigit(v) {
			return false
		}
	}
	return true
}
