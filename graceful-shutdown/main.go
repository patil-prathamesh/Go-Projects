package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
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
				g.cancel()
			})
		}
	}()
}

func work(w http.ResponseWriter, r *http.Request) {
	timer := time.NewTimer(4 * time.Second)
	defer timer.Stop()

	fmt.Println("Work in progress.....")

	select {
	case <-r.Context().Done():
		log.Printf("%s", r.Context().Err())
	case <-timer.C:
		w.Write([]byte("Done"))
	}
}

func heartbeat(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			fmt.Println("heartbeat")
		case <-ctx.Done():
			fmt.Println("heartbeat stopped")
			return
		}
	}
}

func (g *Group) Wait() error {
	g.wg.Wait()
	g.cancel()
	return g.err
}

func main() {
	//Task-1
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	grp, _ := NewGroup(ctx)

	grp.Go(func(ctx context.Context) error {
		heartbeat(ctx)
		return nil
	})

	//Task-2
	mux := http.NewServeMux()
	mux.Handle("GET /work", http.HandlerFunc(work))
	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	grp.Go(func(ctx context.Context) error {
		err := srv.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return err
	})

	//Task-3
	grp.Go(func(ctx context.Context) error {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := srv.Shutdown(shutdownCtx)
		if err != nil {
			srv.Close()
		}
		return err
	})

	err := grp.Wait()
	fmt.Println("Shutdown complete")
	fmt.Printf("%v\n", err)
}
