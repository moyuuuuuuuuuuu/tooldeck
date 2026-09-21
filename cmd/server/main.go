package main

import (
	"context"
	"github.com/moyuuuuuuuuuuu/tooldeck/internal/platform"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	root := os.Getenv("TOOLDECK_DATA_DIR")
	if root == "" {
		root = "data"
	}
	s, e := platform.New(root, os.Getenv("TOOLDECK_ADMIN_PASSWORD"))
	if e != nil {
		log.Fatal(e)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	defer func() { cancel(); s.Wait(); s.Close() }()
	s.Start(ctx)
	addr := os.Getenv("TOOLDECK_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	srv := &http.Server{Addr: addr, Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		c, cc := context.WithTimeout(context.Background(), 45*time.Second)
		defer cc()
		if err := srv.Shutdown(c); err != nil {
			_ = srv.Close()
		}
	}()
	log.Printf("ToolDeck listening on %s", addr)
	if e = srv.ListenAndServe(); e != http.ErrServerClosed {
		log.Print(e)
	}
	cancel()
	<-shutdownDone
}
