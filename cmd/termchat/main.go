package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"termchat/internal/app"
)

const (
	modeServer = "server"
	modeClient = "client"
	modeLocal  = "local"
)

func main() {
	mode, args := parseMode(os.Args[1:])
	flagSet := flag.NewFlagSet("termchat", flag.ExitOnError)
	addr := flagSet.String("addr", envOrDefault("TERMCHAT_ADDR", defaultAddrForMode(mode)), "server listen address")
	path := flagSet.String("path", envOrDefault("TERMCHAT_PATH", "/join"), "websocket join path")
	db := flagSet.String("db", envOrDefault("TERMCHAT_DB_PATH", ""), "sqlite database path (local mode defaults to a per-user path)")
	serverURL := flagSet.String("server-url", envOrDefault("TERMCHAT_SERVER", "wss://termchat-server-al.fly.dev/join"), "server websocket URL (client mode)")
	username := flagSet.String("user", envOrDefault("TERMCHAT_USER", ""), "default username for login prompts")
	quiet := flagSet.Bool("quiet", false, "suppress informational logs")
	flagSet.Parse(args)

	roomKey := ""
	if remaining := flagSet.Args(); len(remaining) > 0 {
		roomKey = remaining[0]
	}

	serverCfg := app.ServerConfig{
		Addr:   *addr,
		Path:   app.NormalizeJoinPath(*path),
		DBPath: *db,
	}
	if serverCfg.DBPath == "" {
		serverCfg.DBPath = app.DefaultDBPath()
	}

	clientCfg := app.ClientConfig{
		ServerURL: *serverURL,
		Username:  *username,
		RoomKey:   roomKey,
	}

	infof := func(format string, args ...interface{}) {
		if *quiet {
			return
		}
		log.Printf(format, args...)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch mode {
	case modeServer:
		err = runServerMode(ctx, serverCfg, infof)
	case modeLocal:
		err = runLocalMode(ctx, serverCfg, clientCfg, infof)
	default:
		err = runClientMode(clientCfg)
	}

	if err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "termchat: %v\n", err)
		os.Exit(1)
	}
}

func runServerMode(ctx context.Context, cfg app.ServerConfig, infof func(string, ...interface{})) error {
	handle, err := app.RunServer(ctx, cfg)
	if err != nil {
		return err
	}
	infof("TermChat server listening on %s (ws path %s, db %s)", handle.Addr(), cfg.Path, cfg.DBPath)
	return handle.Wait()
}

func runClientMode(cfg app.ClientConfig) error {
	if cfg.ServerURL == "" {
		return errors.New("client mode requires --server-url or TERMCHAT_SERVER")
	}
	return app.RunClient(cfg)
}

func runLocalMode(ctx context.Context, serverCfg app.ServerConfig, clientCfg app.ClientConfig, infof func(string, ...interface{})) error {
	if err := os.MkdirAll(filepath.Dir(serverCfg.DBPath), 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	handle, err := app.RunServer(ctx, serverCfg)
	if err != nil {
		return err
	}
	defer stopServer(handle)

	infof("Starting local TermChat server on %s (db %s)", handle.Addr(), serverCfg.DBPath)
	if err := waitForServer(handle.Addr(), 5*time.Second); err != nil {
		return err
	}

	clientCfg.ServerURL = buildWebsocketURL(handle.Addr(), serverCfg.Path)
	infof("Launching client against %s", clientCfg.ServerURL)

	if err := app.RunClient(clientCfg); err != nil {
		return err
	}
	stopServer(handle)
	return handle.Wait()
}

func waitForServer(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("server did not become ready: %w", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func buildWebsocketURL(addr, path string) string {
	path = app.NormalizeJoinPath(path)
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Sprintf("ws://%s%s", addr, path)
	}
	return fmt.Sprintf("ws://%s%s", net.JoinHostPort(host, port), path)
}

func parseMode(args []string) (string, []string) {
	if len(args) == 0 {
		return modeClient, args
	}
	switch strings.ToLower(args[0]) {
	case modeServer, modeClient, modeLocal:
		return strings.ToLower(args[0]), args[1:]
	case "auto": // backward compatibility
		return modeLocal, args[1:]
	}
	return modeClient, args
}

func defaultAddrForMode(mode string) string {
	if mode == modeLocal {
		return "127.0.0.1:0"
	}
	return ":8080"
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func stopServer(handle *app.ServerHandle) {
	if handle == nil {
		return
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = handle.Stop(shutdownCtx)
}
