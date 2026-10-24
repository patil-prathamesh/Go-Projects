package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
)

type Group struct {
	wg     sync.WaitGroup
	once   sync.Once
	ctx    context.Context
	cancel context.CancelFunc
	err    error
}

func NewGroup(ctx context.Context) (*Group, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	group := &Group{ctx: ctx, cancel: cancel}
	return group, ctx
}

func (g *Group) Go(f func(ctx context.Context) error) {
	g.wg.Add(1)

	go func() {
		defer g.wg.Done()
		err := f(g.ctx)
		if err != nil {
			g.once.Do(func() {
				g.err = err
				g.cancel()
			})
		}
	}()
}

func (g *Group) Wait() error {
	g.wg.Wait()
	g.cancel()
	return g.err
}

func check(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	fmt.Println("check for url: ",url)

	if err != nil {
		return fmt.Errorf("from request check %s: %w", url, err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("from response check %s: %w", url, err)
	}
	defer res.Body.Close()

	status := res.StatusCode
	if status != http.StatusOK {
		return fmt.Errorf("check %s: unexpected status %d", url, status)
	}

	return nil
}
