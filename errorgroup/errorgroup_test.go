package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestErrorGroup(t *testing.T) {
	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("request for A", r.RemoteAddr, r.Header.Get("User-Agent"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srvA.Close()

	srvC := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("request for C")
		time.Sleep(300 * time.Millisecond)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srvC.Close()

	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("request for B")
		time.Sleep(1 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srvB.Close()

	srvD := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("request for D")
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()

		select {
		case <-r.Context().Done():
			t.Log(r.Context().Err(), "for D")
		case <-timer.C:
			w.WriteHeader(http.StatusOK)
			return
		}
	}))
	defer srvD.Close()

	urls := []string{srvA.URL, srvB.URL, srvC.URL, srvD.URL}
	start := time.Now()
	grp, _ := NewGroup(context.Background())

	for _, v := range urls {
		grp.Go(func(ctx context.Context) error {
			err := check(ctx, v)
			fmt.Println("return from", v)
			return err
		})
	}

	err := grp.Wait()
	t.Logf("----- %s", err.Error())

	fmt.Println("time required: ", time.Since(start))
}
