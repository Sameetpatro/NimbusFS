package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Logger wraps slog.Logger so we can swap handlers in tests without touching call sites.
type Logger struct {
	// embed the concrete slog logger so we inherit all leveled methods for free
	*slog.Logger
}

// New builds a structured JSON logger with the requested minimum level.
// json handler plays nicely with docker log drivers and centralized log pipelines
func New(level string) *Logger {
	lvl := parseLevel(level)

	// json to stderr because stdout is reserved for cli client output in later phases
	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: lvl,
		// add source file:line in dev-ish levels so we can trace noisy heartbeats quickly
		AddSource: lvl == slog.LevelDebug,
	})

	return &Logger{Logger: slog.New(handler)}
}

// NewWithWriter is like New but writes to w, used in unit tests to capture log lines.
func NewWithWriter(level string, w io.Writer) *Logger {
	lvl := parseLevel(level)
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: lvl})
	return &Logger{Logger: slog.New(handler)}
}

// parseLevel maps config strings to slog levels, defaulting to info when unknown.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		// info is the safe default for production; unknown strings shouldn't silence everything
		return slog.LevelInfo
	}
}

// WithComponent returns a child logger with a stable component field for filtering in Loki/Datadog.
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{Logger: l.With("component", component)}
}

// WithNodeID attaches node identity for correlating storage node logs across restarts.
func (l *Logger) WithNodeID(nodeID string) *Logger {
	return &Logger{Logger: l.With("node_id", nodeID)}
}

// FromContext pulls a request-scoped logger if middleware stored one, else returns the root.
func FromContext(ctx context.Context, fallback *Logger) *Logger {
	if v := ctx.Value(loggerKey{}); v != nil {
		if lg, ok := v.(*Logger); ok {
			return lg
		}
	}
	return fallback
}

// ToContext stores the logger for downstream handlers without global state.
func ToContext(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, l)
}

// loggerKey is an unexported type so external packages can't collide on context keys.
type loggerKey struct{}
