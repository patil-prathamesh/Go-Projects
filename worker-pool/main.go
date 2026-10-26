package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Group struct {
	wg     sync.WaitGroup
	once   sync.Once
	ctx    context.Context
	cancel context.CancelFunc
	err    error
	sem    chan struct{}
}

func NewGroup(ctx context.Context) (*Group, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	grp := &Group{ctx: ctx, cancel: cancel}
	return grp, ctx
}

func (g *Group) SetLimit(n int) {
	if n >= 0 {
		g.sem = make(chan struct{}, n)
	}
}

func (g *Group) Cancel() {
	g.cancel()
}

func (g *Group) Wait() error {
	g.wg.Wait()
	g.cancel()
	return g.err
}

func (g *Group) Go(f func(ctx context.Context) error) {
	if g.sem != nil {
		g.sem <- struct{}{}
	}

	g.wg.Add(1)

	go func() {
		defer g.wg.Done()
		defer func() {
			if g.sem != nil {
				<-g.sem
			}
		}()
		err := f(g.ctx)
		if err != nil {
			g.once.Do(func() {
				g.err = err
				g.Cancel()
			})
		}
	}()
}

type Job struct {
	ID   int
	data string
}

func worker(ctx context.Context, jobs chan Job, wid int) error {
	for {
		select {
		case job, ok := <-jobs:
			if !ok {
				return nil
			}
			time.Sleep(500 * time.Millisecond)
			if job.ID == 7 {
				return errors.New("job 7 occurred...")
			}
			fmt.Printf("Worker %d processing Job %d\n", wid, job.ID)
		case <-ctx.Done():
			for job := range jobs {
				fmt.Printf("Worker %d processing Job %d\n", wid, job.ID)
			}
			return nil
		}
	}
}

func producer(ctx context.Context, jobs chan Job, producerID int) {
	start := (producerID*100 + 1)
	end := start + 99
	for i := start; i <= end; i++ {
		select {
		case jobs <- Job{i, fmt.Sprintf("data-%d", i)}:
		case <-ctx.Done():
			return
		}
	}
}

func main() {
	jobs := make(chan Job, 6)

	grp, ctx := NewGroup(context.Background())

	go func() {
		time.Sleep(20 * time.Second)
		grp.Cancel()
	}()

	for i := 1; i <= 3; i++ {
		id := i
		grp.Go(func(ctx context.Context) error {
			return worker(ctx, jobs, id)
		})
	}

	var wg sync.WaitGroup
	wg.Add(2)

	for i := 0; i <= 1; i++ {
		id := i
		go func() {
			defer wg.Done()
			producer(ctx, jobs, id)
		}()
	}
	wg.Wait()
	close(jobs)

	err := grp.Wait()
	fmt.Println("error:", err)
}
