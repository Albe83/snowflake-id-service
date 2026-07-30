package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/Albe83/id-service/internal/idgen"
	"github.com/Albe83/id-service/internal/server"
)

func parseLogLevel(env string) (slog.Level, error) {
	switch strings.ToLower(env) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid LOG_LEVEL %q: want debug, info, warn, or error", env)
	}
}

func main() {
	level, err := parseLogLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	addr := ":8080"
	srv := server.New(idgen.NewGenerator(nil), logger)
	handler := srv.Routes()

	logger.Info("service starting", "addr", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
