package logging

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

func GetLevel(level string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	case "critical":
		return slog.LevelError + 4, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log level %q", level)
	}
}

func NewLogger(level string) *slog.Logger {
	lvl, err := GetLevel(level)
	opts := &slog.HandlerOptions{Level: lvl}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, opts))

	if err != nil {
		logger.Warn("invalid LOG_LEVEL, using info", "value", level, "error", err)
	}

	return logger
}
