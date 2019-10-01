package lexer

import (
	"context"
	"testing"

	"gotest.tools/assert"
)

func TestToken(t *testing.T) {
	// All the type tests that use _testTokenTypeMaxValue have to skip 0 because of leading iota value
	t.Run("TokenType", func(t *testing.T) {
		t.Run("returns a string value that is 3 characters long", func(t *testing.T) {
			for i := 1; i < int(_testTokenTypeMaxValue); i++ {
				name := TokenType(i).String()
				if len(name) != 3 {
					t.Logf("the name for %s should be 3 characters long...", name)
					t.Fail()
				}
			}
		})
	})

	t.Run("visitor covers all supported token types", func(t *testing.T) {
		v := &counterTokenVisitor{}

		for i := 1; i < int(_testTokenTypeMaxValue); i++ {
			t := &Token{Type: TokenType(i)}

			t.Accept(context.Background(), v)
		}

		assert.Equal(t, int(_testTokenTypeMaxValue)-1, v.count)
	})
}
