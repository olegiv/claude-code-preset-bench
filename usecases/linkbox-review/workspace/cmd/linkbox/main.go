// Command linkbox serves the linkbox API.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"linkbox/internal/api"
	"linkbox/internal/auth"
	"linkbox/internal/store"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "address to listen on")
	keysPath := flag.String("keys", "keys.txt", "file with one key:user pair per line")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(*addr, *keysPath, log); err != nil {
		log.Error("linkbox stopped", "err", err)
		os.Exit(1)
	}
}

func run(addr, keysPath string, log *slog.Logger) error {
	f, err := os.Open(keysPath)
	if err != nil {
		return err
	}
	keys, err := auth.Parse(f)
	f.Close()
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           api.NewServer(store.New(), keys, log).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", addr)
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
