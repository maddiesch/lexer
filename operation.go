package lexer

import (
	"io"
)

// Operation represents a single lexer operation. This is usually a single file.
type Operation interface {
	// Returns the source code's reader.
	SourceReader() io.Reader

	// Returns an ID for the operation that will be appended to tokens.
	ID() string

	// Called before the operation will begin. This is a time to open a file or
	// ensure that required resources are available.
	Prepare() error

	// Called when the operation is completed. This can be used to release any
	// resources retained by the operation.
	Finish() error
}

// OperationResult contains the results of an operation
type OperationResult struct {
	// The ID of the operation passed in
	ID string

	// The tokens generated from the operation
	Tokens []Token
}
