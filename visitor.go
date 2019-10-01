package lexer

import (
	"container/list"
	"context"
	"sync"
)

// Visitor is a type that implements visit methods for tokens
type Visitor interface {
	VisitValue(context.Context, Token) error

	VisitComment(context.Context, Token) error

	VisitKeyword(context.Context, Token) error

	VisitOperator(context.Context, Token) error

	VisitString(context.Context, Token) error

	VisitTerminator(context.Context, Token) error

	VisitNewLine(context.Context, Token) error

	VisitSyntax(context.Context, Token) error
}

// CustomVisitor allows a custom implementation of a visitor to be injected
type CustomVisitor interface {
	Visitor

	SetParent(DefaultVisitor)

	Parent() DefaultVisitor
}

// DefaultVisitor is a wrapper around a Visitor that provides some basic functionality
type DefaultVisitor interface {
	Visitor

	CustomVisitor() CustomVisitor

	// Methods that are available while "walking" with the "Walk" method

	Walk([]Token) error
}

// NewDefaultVisitor returns a new DefaultVisitor instance
func NewDefaultVisitor(visitor CustomVisitor) DefaultVisitor {
	v := &defaultVisitorImpl{sub: visitor}
	visitor.SetParent(v)
	return v
}

type defaultVisitorImpl struct {
	sub CustomVisitor
}

type listElement struct {
	el       *list.Element
	token    Token
	consumed bool
}

type walkContextKeyType struct{}

var (
	walkContextKey = &walkContextKeyType{}
)

// WalkContextFromContext fetches the walk context out of the passed in context
func WalkContextFromContext(ctx context.Context) *WalkContext {
	wc, ok := ctx.Value(walkContextKey).(*WalkContext)
	if !ok {
		return nil
	}

	return wc
}

// WalkContext contains methods available during a walk
type WalkContext struct {
	mu      sync.Mutex
	element *listElement
}

// IsConsumed checks if the current token has been consumed
func (c *WalkContext) IsConsumed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.element == nil {
		return false
	}

	return c.element.consumed
}

// Token returns the current token
func (c *WalkContext) Token() Token {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.element.token
}

// Consume marks a token as consumed for this walk. It will not be visited in the future
func (c *WalkContext) Consume() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.element == nil {
		return
	}

	c.element.consumed = true
}

// Next returns the next token in the list
func (c *WalkContext) Next() (Token, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.element == nil {
		return Token{}, false
	}

	next := c.element.el.Next()
	if next == nil {
		return Token{}, false
	}

	el := next.Value.(*listElement)

	c.element = el

	return el.token, true
}

// Previous returns the previous token in the list
func (c *WalkContext) Previous() (Token, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.element == nil {
		return Token{}, false
	}

	prev := c.element.el.Prev()
	if prev == nil {
		return Token{}, false
	}

	el := prev.Value.(*listElement)

	c.element = el

	return el.token, true
}

func (v *defaultVisitorImpl) Walk(tokens []Token) error {
	walkCtx := &WalkContext{}

	ctx := context.WithValue(context.Background(), walkContextKey, walkCtx)

	list := list.New()
	for _, token := range tokens {
		item := &listElement{token: token}
		item.el = list.PushBack(item)
	}

	for el := list.Front(); el != nil; el = el.Next() {
		item := el.Value.(*listElement)
		if item.consumed {
			continue
		}

		walkCtx.mu.Lock()
		walkCtx.element = item
		walkCtx.mu.Unlock()

		err := item.token.Accept(ctx, v)
		if err != nil {
			return err
		}

		walkCtx.mu.Lock()
		item.consumed = true
		walkCtx.mu.Unlock()
	}

	return nil
}

func (v *defaultVisitorImpl) CustomVisitor() CustomVisitor {
	return v.sub
}

func (v *defaultVisitorImpl) VisitValue(ctx context.Context, t Token) error {
	return v.CustomVisitor().VisitValue(ctx, t)
}

func (v *defaultVisitorImpl) VisitComment(ctx context.Context, t Token) error {
	return v.CustomVisitor().VisitComment(ctx, t)
}

func (v *defaultVisitorImpl) VisitKeyword(ctx context.Context, t Token) error {
	return v.CustomVisitor().VisitKeyword(ctx, t)
}

func (v *defaultVisitorImpl) VisitOperator(ctx context.Context, t Token) error {
	return v.CustomVisitor().VisitOperator(ctx, t)
}

func (v *defaultVisitorImpl) VisitString(ctx context.Context, t Token) error {
	return v.CustomVisitor().VisitString(ctx, t)
}

func (v *defaultVisitorImpl) VisitTerminator(ctx context.Context, t Token) error {
	return v.CustomVisitor().VisitTerminator(ctx, t)
}

func (v *defaultVisitorImpl) VisitNewLine(ctx context.Context, t Token) error {
	return v.CustomVisitor().VisitNewLine(ctx, t)
}

func (v *defaultVisitorImpl) VisitSyntax(ctx context.Context, t Token) error {
	return v.CustomVisitor().VisitSyntax(ctx, t)
}

// WalkTokensWithVisitor performs a visit for every token in the slice
func WalkTokensWithVisitor(ctx context.Context, tokens []Token, visitor Visitor) error {
	for _, token := range tokens {
		err := token.Accept(ctx, visitor)
		if err != nil {
			return err
		}
	}
	return nil
}
