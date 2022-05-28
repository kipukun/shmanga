package group

import (
	"context"
	"sync"
)

type empty struct{}

type Group struct {
	errs chan error
	sem  chan empty

	wg sync.WaitGroup
}

func New(n int) *Group {
	g := new(Group)
	if n > 0 {
		g.errs = make(chan error, n)
	}
	return g
}

func (g *Group) Limit(n int) {
	if n > 0 {
		g.sem = make(chan empty, n)
	}
}

func (g *Group) Do(ctx context.Context, f func() error) {
	g.wg.Add(1)

	if g.sem != nil {
		g.sem <- empty{}
		defer func() { <-g.sem }()
	}

	go func() {
		defer g.wg.Done()
		err := f()
		if err != nil {
			select {
			case g.errs <- err:
			case <-ctx.Done():
				return
			}
			return
		}
	}()
}

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
