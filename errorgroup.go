package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type Group struct {
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once
	err    error
}

func NewGroup(ctx context.Context) (*Group, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	group := &Group{ctx: ctx, cancel: cancel}
	return group, ctx
}

func (g *Group) Go(f func(ctx context.Context) error) {
	fmt.Println("inside Go")
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		err := f(g.ctx)
		if err != nil {
			g.once.Do(func() {
				g.err = err
				fmt.Println(err.Error())
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

func task(ctx context.Context, id int) error {
	timer := time.NewTimer(time.Duration(id) * 300 * time.Millisecond)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		fmt.Printf("task %d: cancelled %s\n", id, ctx.Err().Error())
		return ctx.Err()
	case <-timer.C:
		if id == 3 {
			log.Printf("hit id = 3")
			return fmt.Errorf("ttask %d returns its error at 900ms, and the group cancels the context", id)
		}
		fmt.Printf("task %d: ok\n", id)
		return nil
	}
}

func main() {
	group, _ := NewGroup(context.Background())

	for i := 1; i < 7; i++ {
		group.Go(func(ctx context.Context) error {
			return task(ctx, i)
		})
	}

	fmt.Println("----")
	err := group.Wait()
	fmt.Println(err)

}
