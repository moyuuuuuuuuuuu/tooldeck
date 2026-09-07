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
	s.Start(ctx)
	addr := os.Getenv("TOOLDECK_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	srv := &http.Server{Addr: addr, Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		c, cc := context.WithTimeout(context.Background(), 15*time.Second)
		defer cc()
		srv.Shutdown(c)
	}()
	log.Printf("ToolDeck listening on %s", addr)
	if e = srv.ListenAndServe(); e != http.ErrServerClosed {
		log.Fatal(e)
	}
}
