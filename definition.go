package lexer

import (
	"bytes"
	"sort"
	"unicode/utf8"
)

const (
	// DefinitionPrecedenceHigh is a default value for a high precedence
	// definition
	DefinitionPrecedenceHigh = 10000

	// DefinitionPrecedenceMedium is the default value for a medium precedence
	// definition
	DefinitionPrecedenceMedium = 5000

	// DefinitionPrecedenceLow is the default value for a low precedence
	// definition
	DefinitionPrecedenceLow = 1000

	// DefinitionPrecedenceDefault is the default value for a definition
	// precedence
	//
	// Note that it is also the lowest possible precedence
	DefinitionPrecedenceDefault = 500
)

// Definition allows you to define your own kind of token definition
type Definition interface {
	// Returns the collection of runes that should be matched against
	Runes() []rune

	// Returns the importance of this definition
	//
	// Given a definition for `->` and `-` Giving a precedence of 2 to `->` and 1
	// to `-` will allow the tokenizer to "look ahead" to see if the next
	// character would allow the definition with a higher precedence to be matched
	Precedence() int

	// Check if the def is a "collecting" type where we need to wait until the
	// terminator
	//
	// In the case of a comment, there is no terminator. Only a newline will
	// terminate the collection
	IsCollection() bool

	// Remove the matched runes from the beginning and end of the collection
	StripRunes() bool

	// Create and return a token for the given runes
	Token([]rune, Position) Token

	// is breaking allows a definition to break from a value. The default
	// Definition implementation returns true for any `TokenTypeSyntax`
	IsBreaking() bool
}

// OpenDefinition defines an interface for a Definition that only ends when the
// line ends
type OpenDefinition interface {
	Definition

	// OpenEnded should be a no-op.
	//
	// It's only here to allow the interface to have a definition
	OpenEnded()
}

// TerminatedDefinition defines an interface that a Definition can implement
// that can be used to support multiline comments.
//
// It must return true from the IsCollection method. If it doesn't the end runes
// will not be matched against.
type TerminatedDefinition interface {
	Definition

	EndRunes() []rune
}

// The dBase implements a simple Definition. It can be used to create a fully
// matched token or a token that collects runes until the runs are matched a
// second time.
type dBase struct {
	kind  TokenType
	runes []rune
	prec  int
}

func (d dBase) Runes() []rune {
	return d.runes
}

func (d dBase) IsCollection() bool {
	switch d.kind {
	case TokenTypeString:
		return true
	default:
		return false
	}
}

func (d dBase) StripRunes() bool {
	return d.kind == TokenTypeString
}

func (d dBase) Token(r []rune, p Position) Token {
	return Token{
		Type:     d.kind,
		Value:    string(r),
		Position: p,
	}
}

func (d dBase) Precedence() int {
	return d.prec
}

func (d dBase) IsBreaking() bool {
	return d.kind == TokenTypeSyntax
}

type dComment struct {
	runes []rune
	prec  int
}

// CommentDefinition returns a new OpenDefinition that will collect runes up to
// the end of the line
func CommentDefinition(runes []rune) OpenDefinition {
	return dComment{runes: runes, prec: DefinitionPrecedenceHigh}
}

func (d dComment) OpenEnded() {
}

func (d dComment) Runes() []rune {
	return d.runes
}

func (d dComment) Token(r []rune, p Position) Token {
	return Token{
		Type:     TokenTypeComment,
		Value:    string(r),
		Position: p,
	}
}

func (d dComment) StripRunes() bool {
	return false
}

func (d dComment) IsCollection() bool {
	return true
}

func (d dComment) Precedence() int {
	return d.prec
}

func (d dComment) IsBreaking() bool {
	return true
}

type dMultilineComment struct {
	open  []rune
	close []rune
	prec  int
}

// MultilineCommentDefinition returns a new TerminatedDefinition that will
// generate a multiline comment token.
func MultilineCommentDefinition(open, close []rune) TerminatedDefinition {
	return dMultilineComment{
		open:  open,
		close: close,
		prec:  DefinitionPrecedenceHigh,
	}
}

func (d dMultilineComment) Runes() []rune {
	return d.open
}

func (d dMultilineComment) IsCollection() bool {
	return true
}

func (d dMultilineComment) EndRunes() []rune {
	return d.close
}

func (d dMultilineComment) StripRunes() bool {
	return false
}

func (d dMultilineComment) Token(r []rune, p Position) Token {
	return Token{
		Type:     TokenTypeComment,
		Value:    string(r),
		Position: p,
	}
}

func (d dMultilineComment) Precedence() int {
	return d.prec
}

func (d dMultilineComment) IsBreaking() bool {
	return true
}

// Performs a comparison of all runes in the two passed slices.
func compareRunes(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	return bytes.Equal([]byte(string(a)), []byte(string(b)))
}

// Returns a Definition that is guaranteed to never be matched
func invalidDef() dBase {
	return dBase{
		kind:  TokenTypeValue,
		runes: []rune{utf8.RuneError}, // Ensures that it will never be matched
	}
}

// is used by the lexer to determine if the current passed in definition
// supports multi-line collections.
func defIsMultiline(d Definition) bool {
	if d == nil {
		return false
	}
	_, ok := d.(TerminatedDefinition)

	return ok
}

func defIsOpen(d Definition) bool {
	if d == nil {
		return false
	}
	_, ok := d.(OpenDefinition)

	return ok
}

// Returns the open and close runes for a definition. If the definition is not a
// TerminatedDefinition it will return 2 sets of the same runes.
func getStartEndRunes(d Definition) ([]rune, []rune) {
	r := d.Runes()
	if d, ok := d.(TerminatedDefinition); ok {
		return r, d.EndRunes()
	}
	return r, r
}

// DefinitionSortHandler is the sort function interface for sorting definitions.
//
// It should return false if lhs is "less" than rhs.
type DefinitionSortHandler func(lhs, rhs Definition) bool

// DefinitionRunePrecedenceSorter is the default sorting function for definitions
var DefinitionRunePrecedenceSorter = DefinitionSortHandler(func(lhs, rhs Definition) bool {
	if lhs.Precedence() > rhs.Precedence() {
		return true
	} else if len(lhs.Runes()) > len(rhs.Runes()) {
		return true
	}
	return false
})

type definitionSorter struct {
	definitions []Definition
	fn          DefinitionSortHandler
}

// Sort performs the sorting
func (fn DefinitionSortHandler) Sort(definitions []Definition) {
	sort.Sort(&definitionSorter{definitions: definitions, fn: fn})
}

func (s *definitionSorter) Len() int {
	return len(s.definitions)
}

func (s *definitionSorter) Swap(i, j int) {
	s.definitions[i], s.definitions[j] = s.definitions[j], s.definitions[i]
}

func (s *definitionSorter) Less(i, j int) bool {
	return s.fn(s.definitions[i], s.definitions[j])
}
