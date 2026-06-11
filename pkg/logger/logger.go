package logger

import (
	"context"
	"log/slog"
	"os"
	"time"
)

// this is a wrapper around slog.Logger.
type Logger struct {
	*slog.Logger
}

// creates a new Logger instance.
func New(env string) *Logger {
	var handler slog.Handler

	if env == "local" || env == "development" {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	}

	return &Logger{
		Logger: slog.New(handler),
	}
}

// ExternalCall logs the outcome of an outbound call to an external service using
// the default (configured) logger. service is the target (e.g. "github", "smtp",
// "anthropic"), op is the operation, started marks when the call began (pass
// time.Now() before the call), and err is non-nil on failure. Extra structured
// fields can be supplied via attrs. Only metadata is logged — never request bodies,
// headers, or secrets.
func ExternalCall(ctx context.Context, service, op string, started time.Time, err error, attrs ...any) {
	args := append([]any{
		"service", service,
		"op", op,
		"duration_ms", time.Since(started).Milliseconds(),
	}, attrs...)
	if err != nil {
		slog.ErrorContext(ctx, "external API call failed", append(args, "error", err)...)
		return
	}
	slog.InfoContext(ctx, "external API call", args...)
}
