package lexer

import "context"

// Result is a result object returned by async functions
type Result struct {
	Done  <-chan interface{}
	Error <-chan error
}

func createResult() (*Result, chan interface{}, chan error) {
	done := make(chan interface{}, 1)
	err := make(chan error, 1)

	return &Result{Done: done, Error: err}, done, err
}

// AwaitResult will read the next result from the channel and check if it's an
// error or not. If it is, it will return `nil, err` and if not `value, nil`
func AwaitResult(ctx context.Context, r *Result) (interface{}, error) {
	select {
	case val := <-r.Done:
		return val, nil
	case err := <-r.Error:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
