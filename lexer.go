// Package lexer provides the tokenization functionality of the tootchain.
package lexer

import (
	"bufio"
	"bytes"
	"container/list"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"runtime"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"golang.org/x/sync/semaphore"
)

// Lexer represents a lexer
type Lexer struct {
	mu        sync.RWMutex
	pRegistry []Definition
	pBuffFn   BufferProvider
	pEsc      rune
	pTerm     []rune
	pSemC     int64
	pLogger   *log.Logger
	uSem      *semaphore.Weighted
}

// New creates a new Lexer
func New() *Lexer {
	count := int64(runtime.GOMAXPROCS(0))

	return &Lexer{
		pEsc:      '\\',
		pRegistry: make([]Definition, 0),
		pBuffFn:   NewBuffer,
		pSemC:     count,
		pLogger:   log.New(ioutil.Discard, "", 0),
		pTerm:     []rune{},
		uSem:      semaphore.NewWeighted(count),
	}
}

// SetEscape sets the escape character. The default is `\`
func (l *Lexer) SetEscape(r rune) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.pEsc = r
}

// SetBufferProvider sets the lexer's buffer provider function.
func (l *Lexer) SetBufferProvider(p BufferProvider) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.pBuffFn = p
}

// SetLogger assigns the new logger for the lexer
func (l *Lexer) SetLogger(lg *log.Logger) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.pLogger = lg
}

// SetTerminator sets the list of terminator characters other than whitespace.
func (l *Lexer) SetTerminator(r ...rune) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.pTerm = r
}

// RegisterSingleRunes allows multiple single characters tokens to be registered
// for a single type. In one call
//
// lex.RegisterSingleRunes(lexer.TokenTypeSyntax, '(', ')', '{', '}')
func (l *Lexer) RegisterSingleRunes(kind TokenType, runes ...rune) {
	for _, r := range runes {
		l.RegisterDefinitions(dBase{
			runes: []rune{r},
			kind:  kind,
			prec:  DefinitionPrecedenceDefault,
		})
	}
}

// Register adds a set of runes to be matched against for the specific types
func (l *Lexer) Register(kind TokenType, runes []rune, precedence ...int) {
	prec := DefinitionPrecedenceDefault
	if len(precedence) > 0 {
		prec = precedence[0]
	}
	l.RegisterDefinitions(dBase{
		runes: runes,
		kind:  kind,
		prec:  prec,
	})
}

// RegisterDefinitions registers the set of definitions with the lexer
func (l *Lexer) RegisterDefinitions(def ...Definition) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, def := range def {
		if len(def.Runes()) == 0 {
			panic(fmt.Errorf("attempted to register an empty definition: %v", def))
		}
		l.pRegistry = append(l.pRegistry, def)
	}
}

// Run generates tokens from the given operation
func (l *Lexer) Run(o Operation) ([]Token, error) {
	return AwaitCompletion(l.RunWithContext(context.Background(), o))
}

// RunWithContext performs an asynchronous operation. The channel returned will
// receive either an OperationResult or an error
func (l *Lexer) RunWithContext(ctx context.Context, o Operation) *Result {
	// Ensure that the registry is sorted properly
	l.mu.Lock()
	DefinitionRunePrecedenceSorter.Sort(l.pRegistry)
	l.mu.Unlock()

	result, dChan, eChan := createResult()

	go func(ctx context.Context, o Operation) {
		id := o.ID()

		l.logger().Printf("[%s] enqueue lexer operation", id)

		if err := l.uSem.Acquire(ctx, 1); err != nil {
			eChan <- err
			return
		}
		defer l.uSem.Release(1)

		l.logger().Printf("[%s] lexer operation starting", id)

		l.mu.RLock()
		tokens, id, err := l.pExecute(ctx, id, o)
		l.mu.RUnlock()

		if err != nil {
			eChan <- err
		} else {
			dChan <- OperationResult{
				Tokens: tokens,
				ID:     id,
			}
		}
	}(ctx, o)

	return result
}

func (l *Lexer) logger() *log.Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.pLogger
}

func (l *Lexer) pIsTerm(r rune) bool {
	for _, t := range l.pTerm {
		if t == r {
			return true
		}
	}
	return false
}

func (l *Lexer) pExecute(ctx context.Context, id string, o Operation) ([]Token, string, error) {
	if err := o.Prepare(); err != nil {
		return []Token{}, id, err
	}

	scanner := bufio.NewScanner(o.SourceReader())
	scanner.Split(bufio.ScanLines) // lines

	loc := mLoc(0, 0)

	var wasEsc bool
	var colDef Definition

	tokens := make([]Token, 0)

	buffer := l.pBuffFn()
	if buffer == nil {
		return tokens, id, fmt.Errorf("failed to create a new buffer")
	}

	for scanner.Scan() {
		loc.Line++
		loc.Column = 0

		l.pLogger.Printf("[%s] beginning new line %s", id, loc)

		scanner := bufio.NewScanner(bytes.NewReader(scanner.Bytes()))
		scanner.Split(bufio.ScanRunes) // UTF-8 runes

		// Using a linked list so the look-ahead is easier for precedence matching
		characters := list.New()

		// Scan each character in the line into the list
		for scanner.Scan() {
			loc.Column++

			ru, _ := utf8.DecodeLastRune(scanner.Bytes())

			pos := mPosition(id, loc.Line, LocationValueInvalid, loc.Column, LocationValueInvalid)

			// We don't know how to handle this error, so we wrap it in an encoding
			// error definition and return that as the error to the caller.
			if ru == utf8.RuneError {
				// We don't really care about the finished state because we have already
				// failed.
				o.Finish()

				return []Token{}, id, &Error{
					Position: pos,
					Message:  "lexer encountered an invalid UTF-8 character",
				}
			}

			characters.PushBack(&listItem{
				ru:  ru,
				pos: pos,
			})
		}

		// Enumerate through all the items in the list
		for element := characters.Front(); element != nil; element = element.Next() {
			item := element.Value.(*listItem)

			if item.consumedByLookahead {
				continue
			}

			ru := item.ru

			// If the character is an escape and the previous character didn't escape
			// the escape then continue to the next character
			if ru == l.pEsc && !wasEsc {
				// Set the state that the previous character was an escape
				wasEsc = true

				// Skip ahead, nothing more to do
				continue
			}

			// Space is a breaking character, we'll flush the buffer
			//
			// If the buffer is currently collecting runes looking for terminating
			// runes we do not skip the break
			if unicode.IsSpace(ru) && colDef == nil {
				tokens = flushBuffer(l.pLogger, id, buffer, invalidDef(), loc, tokens)
				continue
			}

			// If we encounter a terminator and it was not escaped and we aren't
			// collecting runes, then we can flush the buffer. Write a terminating
			// token, and continue
			if !wasEsc && colDef == nil && l.pIsTerm(ru) {
				// Flush the current buffer. with an end column 1 character ago.
				tokens = flushBuffer(l.pLogger, id, buffer, nil, mLoc(loc.Line, loc.Column-1), tokens)
				buffer.Write(loc, ru)
				tokens = flushBuffer(l.pLogger, id, buffer, &dBase{kind: TokenTypeTerminator}, mLoc(loc.Line, loc.Column-1), tokens)
				continue
			}

			// Write the rune into the buffer
			buffer.Write(item.pos.Range.Start, ru)

			// Check if the end matches. If we escaped it would be impossible for the
			// the end of the collection to be satisfied.
			if !wasEsc && isColAndCompleted(buffer, colDef) {
				// Collection was satisfied... Continue
				tokens = flushBuffer(l.pLogger, id, buffer, colDef, loc, tokens)
				// Reset the collection definition
				colDef = nil
				continue
			}

			// Reset the last escape value
			wasEsc = false

			for _, def := range l.pRegistry {
				dRunes := def.Runes()

				if def.IsBreaking() && colDef == nil {
					newBuffer := checkForBreakingDefinitionWithLookAhead(l.pBuffFn(), dRunes, element)
					if newBuffer != nil {
						// The breaking definition is the entire value... So we don't need to create a value token
						if !compareRunes(buffer.Runes(), newBuffer.Runes()) {
							// This will always trigger on the first rune so we only need to walk back 1 insert
							// If this empties the buffer that's fine. The flushBuffer handles an empty buffer
							buffer.Delete(1)

							// Flush the existing buffer as a value
							tokens = flushBuffer(l.pLogger, id, buffer, colDef, mLoc(loc.Line, loc.Column-1), tokens)
						}

						// Reset the old buffer so it's empty
						buffer.Reset()

						if def.IsCollection() {
							l.pLogger.Printf("[%s] starting a collection %s", id, loc)

							// Copy what we have in the new buffer into the buffer so it can be used to build the
							// collection
							for _, r := range newBuffer.Runes() {
								buffer.Write(newBuffer.StartLocation(), r)
							}

							colDef = def
						} else {
							// Flush the new buffer as the breaking definition
							tokens = flushBuffer(l.pLogger, id, newBuffer, def, loc, tokens)
						}

						break
					}
				}

				if !compareWithLookaheadAndConsume(buffer, dRunes, element) {
					continue // They don't match
				}

				// The definition is a collection of runes
				if def.IsCollection() {
					l.pLogger.Printf("[%s] starting a collection %s", id, loc)
					colDef = def
				} else {
					l.pLogger.Printf("[%s] matched definition '%s' %s", id, string(dRunes), loc)
					tokens = flushBuffer(l.pLogger, id, buffer, def, loc, tokens)
				}

				break
			}
		}

		if defIsMultiline(colDef) {
			loc.Column++

			// Write the newline into the buffer for the multiline definition
			buffer.Write(loc, '\n')
		} else {
			tokens = flushBuffer(l.pLogger, id, buffer, colDef, loc, tokens)
			if loc.Column > 1 {
				tokens = append(tokens, Token{
					Type:     TokenTypeNewLine,
					Value:    "\n",
					Position: mPosition(id, loc.Line, loc.Line, loc.Column, loc.Column+1),
				})
			}
			colDef = nil
		}
	}

	tokens = flushBuffer(l.pLogger, id, buffer, colDef, loc, tokens)

	if err := o.Finish(); err != nil {
		return []Token{}, id, err
	}

	return tokens, id, nil
}

// Wait waits for all the operations in flight to finish
func (l *Lexer) Wait() error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.pWait()
}

// pWait must not be called outside of the lexers lock.
func (l *Lexer) pWait() error {
	err := l.uSem.Acquire(context.Background(), l.pSemC)
	if err != nil {
		return err
	}

	l.uSem.Release(l.pSemC)

	return nil
}

func isColAndCompleted(b Buffer, d Definition) bool {
	if d == nil {
		return false
	}
	if defIsOpen(d) {
		return false
	}
	runes := b.Runes()

	_, end := getStartEndRunes(d)

	if len(runes) < len(end) {
		return false
	}

	return compareRunes(runes[len(runes)-len(end):], end)
}

type listItem struct {
	ru                  rune
	pos                 Position
	consumedByLookahead bool
}

// checkForBreakingDefinitionWithLookAhead will check if the breaking definition
// matched using a new buffer for the look-ahead
func checkForBreakingDefinitionWithLookAhead(buf Buffer, def []rune, el *list.Element) Buffer {
	item := el.Value.(*listItem)

	buf.Write(item.pos.Range.Start, item.ru)

	if compareWithLookaheadAndConsume(buf, def, el) {
		return buf
	}

	return nil
}

func compareWithLookaheadAndConsume(buf Buffer, def []rune, el *list.Element) bool {
	buffered := buf.Runes()

	// The buffer contains more characters than the definition requires.
	// There is no way this could ever match even with a look-ahead.
	if len(buffered) > len(def) {
		return false
	}

	// Check if any of the buffers current characters don't match the definitions
	// requirements.
	for i := 0; i < len(buffered); i++ {
		if buffered[i] != def[i] {
			return false
		}
	}
	// Create the new list
	items := make([]*listItem, 0, len(def))

	lookaheadMatch := true

	next := el.Next()

	sub := make([]rune, len(def))
	copy(sub, def)

	for _, ru := range sub[len(buffered):] {
		// Not enough characters left in the line
		if next == nil {
			return false
		}

		item := next.Value.(*listItem)

		// Lookahead failed
		if item.ru != ru {
			return false
		}

		items = append(items, item)

		next = next.Next()
	}

	if lookaheadMatch {
		for _, i := range items {
			i.consumedByLookahead = true
			buf.Write(i.pos.Range.Start, i.ru)
		}
	}

	return lookaheadMatch
}

// Flushes the buffer and creates a new token appending it to the list of passed
// tokens and then returns the new list
//
// If the buffer is empty, nothing will happen.
func flushBuffer(l *log.Logger, id string, b Buffer, d Definition, end Location, t []Token) []Token {
	start := b.StartLocation()
	runes := b.Runes()

	b.Reset()

	// There are no runes so there is nothing to do
	if len(runes) == 0 {
		return t
	}

	msg := []string{
		"Flushing buffer",
		fmt.Sprintf("Position (%s, %s): '%s'", start, end, string(runes)),
	}

	if d == nil {
		msg = append(msg, "Unmatched... Creating a value")
		d = dBase{kind: TokenTypeValue}
	}

	if d.StripRunes() {
		msg = append(msg)

		msg = append(msg, "Stripping definition runes...")

		start, end := getStartEndRunes(d)

		runes = runes[len(start) : len(runes)-len(end)]
	}

	pos := Position{
		ID: id,
		Range: Range{
			Start: start,
			End:   end,
		},
	}

	token := d.Token(runes, pos)

	msg = append(msg, fmt.Sprintf("Adding token (%s)", token.Type))

	{
		builder := strings.Builder{}

		builder.WriteRune('[')
		builder.WriteString(id)
		builder.WriteRune(']')

		for _, m := range msg {
			builder.WriteRune('\n')
			builder.WriteString(strings.Repeat(" ", 7))
			builder.WriteRune(' ')
			builder.WriteString(m)
		}

		l.Println(builder.String())
	}

	return append(t, token)
}

// Error represents an error encountered during lexing
type Error struct {
	Position Position
	Message  string
}

func (e *Error) Error() string {
	return fmt.Sprintf("")
}

// Returns the minimum value from the list of integers
func intMax(n ...int) int {
	if len(n) == 0 {
		return 0
	}
	cur := n[0]

	for _, i := range n {
		if i > cur {
			cur = i
		}
	}

	return cur
}

// Returns the maximum value from the list of integers
func intMin(n ...int) int {
	if len(n) == 0 {
		return 0
	}
	cur := n[0]

	for _, i := range n {
		if i < cur {
			cur = i
		}
	}

	return cur
}

func reverse(a []rune) {
	for i := len(a)/2 - 1; i >= 0; i-- {
		opp := len(a) - 1 - i
		a[i], a[opp] = a[opp], a[i]
	}
}

// AwaitCompletion takes the channel returned by Lexer's RunWithContext function
// and waits for a signal. It will return the tokens or an error.
func AwaitCompletion(c *Result) ([]Token, error) {
	result, err := AwaitResult(context.Background(), c)
	if err != nil {
		return nil, err
	}

	return result.(OperationResult).Tokens, nil
}
