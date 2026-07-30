package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

const (
	defaultPort = 8080
	minPort     = 1
	maxPort     = 65535
)

type Config struct {
	Addr     string
	LogLevel slog.Level
}

func Load() (Config, error) {
	port := defaultPort
	if v := os.Getenv("PORT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < minPort || n > maxPort {
			return Config{}, fmt.Errorf("invalid PORT %q: want integer %d-%d", v, minPort, maxPort)
		}
		port = n
	}

	level, err := parseLogLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		return Config{}, err
	}

	return Config{
		Addr:     ":" + strconv.Itoa(port),
		LogLevel: level,
	}, nil
}

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
