package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"termchat/internal/app"
)

func main() {
	addr := flag.String("addr", envOrDefault("TERMCHAT_ADDR", ":8080"), "server listen address")
	path := flag.String("path", envOrDefault("TERMCHAT_PATH", "/join"), "websocket join path")
	dbPath := flag.String("db", envOrDefault("TERMCHAT_DB_PATH", app.DefaultDBPath()), "sqlite database path")
	flag.Parse()

	serverCfg := app.ServerConfig{
		Addr:   *addr,
		Path:   app.NormalizeJoinPath(*path),
		DBPath: *dbPath,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	handle, err := app.RunServer(ctx, serverCfg)
	if err != nil {
		log.Fatalf("start server: %v", err)
	}
	log.Printf("TermChat server listening on %s (ws path %s)", handle.Addr(), serverCfg.Path)

	if err := handle.Wait(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
