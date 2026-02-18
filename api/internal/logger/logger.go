// Package logger provides structured logging using zerolog,
// matching the format used in deeplibby.
package logger

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type contextKey string

const requestIDKey contextKey = "request_id"

const milliTimeFormat = "2006-01-02T15:04:05.000Z07:00"

// Init initializes the global logger with proper configuration based on environment.
func Init() {
	zerolog.TimeFieldFormat = milliTimeFormat
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }

	const callerWidth = 30
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		path := fmt.Sprintf("%s:%d", filepath.Base(file), line)
		if len(path) >= callerWidth {
			return path[len(path)-callerWidth:]
		}
		return path + strings.Repeat(" ", callerWidth-len(path))
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	var output io.Writer = zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: milliTimeFormat,
		NoColor:    !isDevelopmentMode(),
	}

	if logFile := os.Getenv("LOG_FILE"); logFile != "" {
		f, ferr := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if ferr == nil {
			output = io.MultiWriter(output, f)
		}
	}

	log.Logger = log.Output(output).With().Caller().Logger()

	log.Info().
		Str("level", level.String()).
		Bool("dev", isDevelopmentMode()).
		Msg("Logger initialized")
}

func isDevelopmentMode() bool {
	return os.Getenv("DEV") == "true" ||
		os.Getenv("DEV_MODE") == "true" ||
		os.Getenv("DEVELOPMENT") == "true"
}

// Get returns the global logger instance.
func Get() zerolog.Logger {
	return log.Logger
}

// NewRequestID generates a cryptographically secure random 8-character alphanumeric string.
func NewRequestID() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 8

	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return fmt.Sprintf("req%06d", time.Now().UnixNano()%1000000)
	}

	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b)
}

// WithRequestID returns a new context with the given request ID stored.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFromContext extracts the request ID from context, or empty string.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// ForRequest returns a logger enriched with the request ID from context.
func ForRequest(ctx context.Context) zerolog.Logger {
	id := RequestIDFromContext(ctx)
	if id == "" {
		return log.Logger
	}
	return log.Logger.With().Str("requestId", id).Logger()
}

// LogRequest logs the request body at debug level, truncating if too long.
func LogRequest(logger zerolog.Logger, body []byte) {
	if len(body) == 0 {
		return
	}
	if len(body) > 1000 {
		logger.Debug().Str("request_body", string(body[:1000])).Bool("truncated", true).Msg("Request body")
	} else {
		logger.Debug().Str("request_body", string(body)).Msg("Request body")
	}
}

// LogResponse logs the response body at debug level, truncating if too long.
func LogResponse(logger zerolog.Logger, body []byte) {
	if len(body) == 0 {
		return
	}
	if len(body) > 1000 {
		logger.Debug().Str("response", string(body[:1000])).Bool("truncated", true).Msg("Response body")
	} else {
		logger.Debug().Str("response", string(body)).Msg("Response body")
	}
}
