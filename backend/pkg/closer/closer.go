// Package closer provides a utility for graceful shutdown management.
// It allows registering cleanup functions that will be executed concurrently or sequentially on shutdown.
package closer

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ErrCloseFailed indicates that one or more cleanup functions returned an error.
var ErrCloseFailed = errors.New("closer finished with errors")

// Func is a signature for a cleanup function.
type Func func(ctx context.Context) error

// Closer manages a list of cleanup functions to be executed during shutdown.
type Closer struct {
	mu    sync.Mutex
	funcs []Func
}

// New creates and returns a new Closer instance.
func New() *Closer {
	return &Closer{}
}

// Add registers a new cleanup function to be called during Close.
func (c *Closer) Add(f Func) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.funcs = append(c.funcs, f)
}

// Close executes all registered cleanup functions.
// If any of them return an error, it collects them and returns a wrapped ErrCloseFailed.
func (c *Closer) Close(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errs []error

	for _, f := range c.funcs {
		if err := f(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("closer.Close error: %w (details: %v)", ErrCloseFailed, errs)
	}
	return nil
}
