package group

import (
	"context"
	"sync"
)

type empty struct{}

// Group orchestrates goroutines that return errors.
type Group struct {
	errs chan error
	sem  chan empty
	wg   sync.WaitGroup

	cancel context.CancelFunc
}

func WithContext(ctx context.Context) (*Group, context.Context) {
	g := new(Group)
	ctx, cancel := context.WithCancel(ctx)
	g.cancel = cancel
	return g, ctx
}

// Limit creates a limit on the max concurrent goroutines.
func (g *Group) Limit(n int) {
	if n > 0 {
		g.sem = make(chan empty, n)
	}
}

// Do runs the given function f under the context ctx in the group g.
// Do blocks until there is enough room as defined by Limit.
// Do is a no-op if ctx is canceled.
func (g *Group) Do(ctx context.Context, f func() error) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	if g.errs == nil {
		g.errs = make(chan error, 1)
	}

	g.wg.Add(1)
	go func() {
		defer g.wg.Done()

		if g.sem != nil {
			g.sem <- empty{}
			defer func() { <-g.sem }()
		}

		err := f()
		if err != nil {
			select {
			case g.errs <- err:
				g.cancel()
			case <-ctx.Done():
			}
			return
		}
	}()
}

// Wait blocks until either ctx is canceled, a goroutine errors
// or all goroutines are finished, whichever comes first.
// Wait returns the context error or goroutine error, or
// nil if no error occurred.
func (g *Group) Wait(ctx context.Context) error {

	done := make(chan struct{}, 1)
	go func() {
		g.wg.Wait()
		done <- struct{}{}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-g.errs:
		return err
	case <-done:
		return nil
	}
}
