package lexer_test

import (
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
	"testing"

	lexer "github.com/maddiesch/lexer"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

func createLexer() *lexer.Lexer {
	lex := lexer.New()

	lex.SetTerminator(';')

	lex.Register(lexer.TokenTypeKeyword, []rune{'s', 'y', 's', ':'})
	lex.Register(lexer.TokenTypeKeyword, []rune{'e', 'n', 't', 'r', 'y', ':'})
	lex.Register(lexer.TokenTypeKeyword, []rune{'l', 'e', 't'})
	lex.Register(lexer.TokenTypeKeyword, []rune{'r', 'e', 't'})
	lex.Register(lexer.TokenTypeKeyword, []rune{'a', 's'})

	lex.Register(lexer.TokenTypeOperator, []rune{'+'})
	lex.Register(lexer.TokenTypeOperator, []rune{'-'})
	lex.Register(lexer.TokenTypeOperator, []rune{'*'})
	lex.Register(lexer.TokenTypeOperator, []rune{'/'})
	lex.Register(lexer.TokenTypeOperator, []rune{'='})

	lex.Register(lexer.TokenTypeSyntax, []rune{'.'})
	lex.Register(lexer.TokenTypeSyntax, []rune{','})
	lex.Register(lexer.TokenTypeSyntax, []rune{'('})
	lex.Register(lexer.TokenTypeSyntax, []rune{')'})
	lex.Register(lexer.TokenTypeSyntax, []rune{'{'})
	lex.Register(lexer.TokenTypeSyntax, []rune{'}'})
	lex.Register(lexer.TokenTypeSyntax, []rune{'-', '>'})

	lex.Register(lexer.TokenTypeString, []rune{'"'})

	lex.RegisterDefinitions(
		lexer.CommentDefinition([]rune{'/', '/'}),
		lexer.MultilineCommentDefinition([]rune{'/', '*'}, []rune{'*', '/'}),
	)

	return lex
}

func stringOp(src string) lexer.Operation {
	return &stringSourceOperation{Src: src}
}

type stringSourceOperation struct {
	Src string
}

// SourceReader returns the reader for the source code
func (o *stringSourceOperation) SourceReader() io.Reader {
	return strings.NewReader(o.Src)
}

// ID returns a unique identifier for the source code
func (o *stringSourceOperation) ID() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(o.Src)))
}

// Prepare is called before the tokenization begins
func (o *stringSourceOperation) Prepare() error {
	return nil
}

// Finish is called after the tokenization is completed
//
// This method is even called after errors, and in that case any error returned
// by this function will de discarded
func (o *stringSourceOperation) Finish() error {
	return nil
}

func TestLexer(t *testing.T) {
	lex := createLexer()

	t.Run("given a string with invalid UTF-8 characters", func(t *testing.T) {
		input := string([]byte{108, 101, 116, 32, 97, 32, 61, 32, 34, 239, 191, 189, 34})

		_, err := lex.Run(stringOp(input))

		e, ok := err.(*lexer.Error)

		require.True(t, ok)

		assert.Equal(t, "lexer encountered an invalid UTF-8 character", e.Message)
	})

	t.Run("parses a simple assignment", func(t *testing.T) {
		tokens, err := lex.Run(stringOp(`let name = "Maddie"`))

		require.NoError(t, err)

		require.Equal(t, 5, len(tokens))

		assert.Equal(t, lexer.TokenTypeKeyword, tokens[0].Type)
		assert.Equal(t, "let", tokens[0].Value)

		assert.Equal(t, lexer.TokenTypeValue, tokens[1].Type)
		assert.Equal(t, "name", tokens[1].Value)

		assert.Equal(t, lexer.TokenTypeOperator, tokens[2].Type)
		assert.Equal(t, "=", tokens[2].Value)

		assert.Equal(t, lexer.TokenTypeString, tokens[3].Type)
		assert.Equal(t, "Maddie", tokens[3].Value)

		assert.Equal(t, lexer.TokenTypeNewLine, tokens[4].Type)
	})

	t.Run("parses with an escaped string", func(t *testing.T) {
		tokens, err := lex.Run(stringOp(`let s = "He \"said\" that!"`))

		require.NoError(t, err)

		require.Equal(t, 5, len(tokens))

		assert.Equal(t, lexer.TokenTypeString, tokens[3].Type)
		assert.Equal(t, `He "said" that!`, tokens[3].Value)
	})

	t.Run("it parses a comment", func(t *testing.T) {
		src := "// This is an opening comment\nlet a = 1; let b = 2; // With a trailing comment"

		tokens, err := lex.Run(stringOp(src))

		require.NoError(t, err)

		require.Equal(t, 14, len(tokens))
	})

	t.Run("it parses a multi-line comment", func(t *testing.T) {
		src := "// This is an opening comment\n/*\nExample\n  let name = \"Maddie\";\n*/\nlet a = 1;\n\n"

		tokens, err := lex.Run(stringOp(src))

		require.NoError(t, err)

		require.Equal(t, 10, len(tokens))
	})

	t.Run("it will match a definition as a breaker during a value", func(t *testing.T) {
		lex := lexer.New()
		lex.Register(lexer.TokenTypeKeyword, []rune{'a', 'd', 'd'})
		lex.Register(lexer.TokenTypeOperator, []rune{'+'})
		lex.RegisterSingleRunes(lexer.TokenTypeSyntax, '(', ')', '{', '}', ',')

		src := "add(a, b) { a + b }"

		tokens, err := lex.Run(stringOp(src))

		require.NoError(t, err)

		require.Equal(t, 12, len(tokens))

		assert.Equal(t, ",", tokens[3].Value)
		assert.Equal(t, ")", tokens[5].Value)
	})

	t.Run("it performs look ahead parsing for precedence", func(t *testing.T) {
		lex := lexer.New()
		lex.Register(lexer.TokenTypeKeyword, []rune{'h', 'i'})
		lex.Register(lexer.TokenTypeKeyword, []rune{'h', 'i', 'g', 'h'}, lexer.DefinitionPrecedenceLow)
		lex.Register(lexer.TokenTypeKeyword, []rune{'h', 'i', 'g', 'h', 'e', 'r'}, lexer.DefinitionPrecedenceMedium)
		lex.Register(lexer.TokenTypeKeyword, []rune{'h', 'i', 'g', 'h', 'e', 's', 't'}, lexer.DefinitionPrecedenceHigh)

		src := "high\nhigher\nhi\nhighest\n"

		tokens, err := lex.Run(stringOp(src))

		require.NoError(t, err)

		require.Equal(t, 8, len(tokens))

		assert.Equal(t, "high", tokens[0].Value)
		assert.Equal(t, "higher", tokens[2].Value)
		assert.Equal(t, "hi", tokens[4].Value)
	})

	t.Run("it should not strip the leading matches from a comment", func(t *testing.T) {
		lex := lexer.New()
		lex.RegisterDefinitions(
			lexer.CommentDefinition([]rune{'/', '/'}),
		)

		src := "// Opening Comment"

		tokens, err := lex.Run(stringOp(src))

		require.NoError(t, err)

		require.Equal(t, 2, len(tokens))

		assert.Equal(t, "// Opening Comment", tokens[0].Value)
	})
}

func BenchmarkLexerOperation(b *testing.B) {
	lex := createLexer()
	src := `
// Add takes two numbers and performs an addition operation on them and returns the result
let add = -> (a, b) { ret a + b }

// Entry to our program
entry: -> {
	// Set the value of value equal to the result of the add function call
	let value = add(5, 8)

	// Perform the system call to print the value of the string to Stdout
	sys:print('add(5, 8) = ${value}')
}
	`
	op := stringOp(src)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		lex.Run(op)
	}
}
