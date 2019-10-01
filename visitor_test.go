package lexer

import (
	"context"
	"testing"

	"gotest.tools/assert"
)

type counterTokenVisitor struct {
	types map[TokenType]int
	def   DefaultVisitor
	count int
}

func (n *counterTokenVisitor) countType(t TokenType) {
	if n.types == nil {
		n.types = make(map[TokenType]int, 0)
	}
	n.types[t]++
	n.count++
}

func (n *counterTokenVisitor) VisitValue(_ context.Context, t Token) error {
	n.countType(t.Type)
	return nil
}

func (n *counterTokenVisitor) VisitComment(_ context.Context, t Token) error {
	n.countType(t.Type)
	return nil
}

func (n *counterTokenVisitor) VisitKeyword(_ context.Context, t Token) error {
	n.countType(t.Type)
	return nil
}

func (n *counterTokenVisitor) VisitOperator(_ context.Context, t Token) error {
	n.countType(t.Type)
	return nil
}

func (n *counterTokenVisitor) VisitString(_ context.Context, t Token) error {
	n.countType(t.Type)
	return nil
}

func (n *counterTokenVisitor) VisitTerminator(_ context.Context, t Token) error {
	n.countType(t.Type)
	return nil
}

func (n *counterTokenVisitor) VisitNewLine(_ context.Context, t Token) error {
	n.countType(t.Type)
	return nil
}

func (n *counterTokenVisitor) VisitSyntax(ctx context.Context, t Token) error {
	walk := WalkContextFromContext(ctx)
	if walk != nil {
		walk.Next()
		walk.Consume()
	}
	n.countType(t.Type)
	return nil
}

func (n *counterTokenVisitor) SetParent(def DefaultVisitor) {
	n.def = def
}

func (n *counterTokenVisitor) Parent() DefaultVisitor {
	return n.def
}

func TestWalk(t *testing.T) {
	t.Run("given a walk we visit every token", func(t *testing.T) {
		tokens := []Token{
			Token{Type: TokenTypeKeyword},
			Token{Type: TokenTypeKeyword},
			Token{Type: TokenTypeKeyword},
			Token{Type: TokenTypeKeyword},
			Token{Type: TokenTypeKeyword},
		}

		counter := &counterTokenVisitor{}

		def := NewDefaultVisitor(counter)

		def.Walk(tokens)

		assert.Equal(t, 5, counter.count)
	})

	t.Run("given a walk that the next token after syntax as consumed", func(t *testing.T) {
		tokens := []Token{
			Token{Type: TokenTypeSyntax},
			Token{Type: TokenTypeKeyword},
			Token{Type: TokenTypeKeyword},
			Token{Type: TokenTypeSyntax},
			Token{Type: TokenTypeKeyword},
			Token{Type: TokenTypeKeyword},
			Token{Type: TokenTypeSyntax},
			Token{Type: TokenTypeKeyword},
		}

		counter := &counterTokenVisitor{}

		def := NewDefaultVisitor(counter)

		def.Walk(tokens)

		assert.Equal(t, 5, counter.count)

		assert.Equal(t, 2, counter.types[TokenTypeKeyword])
		assert.Equal(t, 3, counter.types[TokenTypeSyntax])
	})
}
