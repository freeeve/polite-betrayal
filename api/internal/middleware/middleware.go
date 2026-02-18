package middleware

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/efreeman/polite-betrayal/api/internal/logger"
)

// Logger logs each request with a unique request ID, method, path, status, and duration.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := logger.NewRequestID()

		ctx := logger.WithRequestID(r.Context(), requestID)
		r = r.WithContext(ctx)

		logCtx := logger.Get().With().
			Str("requestId", requestID).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Logger()

		// Log request body at debug level.
		if r.Body != nil {
			bodyBytes, err := io.ReadAll(r.Body)
			if err == nil && len(bodyBytes) > 0 {
				logger.LogRequest(logCtx, bodyBytes)
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		}

		logCtx.Info().
			Interface("queryParams", r.URL.Query()).
			Msg("Request received")

		rw := &responseWriter{ResponseWriter: w, buf: &bytes.Buffer{}, status: http.StatusOK}
		next.ServeHTTP(rw, r)

		logger.LogResponse(logCtx, rw.buf.Bytes())
		logCtx.Info().
			Int("status", rw.status).
			Dur("durationMs", time.Since(start)).
			Msg("Request completed")
	})
}

// CORS adds Cross-Origin Resource Sharing headers.
func CORS(allowedOrigins string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigins)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// JSON sets the Content-Type header to application/json for all responses.
func JSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// Chain applies middleware in order (first applied = outermost).
func Chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// responseWriter wraps http.ResponseWriter to capture response body and status.
type responseWriter struct {
	http.ResponseWriter
	buf    *bytes.Buffer
	status int
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.buf.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *responseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Hijack implements http.Hijacker so WebSocket upgrades work through the logging middleware.
func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
}
