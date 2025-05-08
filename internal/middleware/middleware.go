// Package middleware provides HTTP middleware functions
package middleware

import (
	"bufio"
	"errors"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"
)

// Middleware defines a function to process http requests
type Middleware func(http.Handler) http.Handler

// Chain applies middlewares to a http.Handler
func Chain(handler http.Handler, middlewares ...Middleware) http.Handler {
	for _, middleware := range middlewares {
		handler = middleware(handler)
	}
	return handler
}

// Logger returns a middleware that logs HTTP requests
func Logger() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Don't log requests for static files
			if !strings.HasPrefix(r.URL.Path, "/api") &&
				!strings.HasPrefix(r.URL.Path, "/ws") &&
				r.URL.Path != "/health" {
				next.ServeHTTP(w, r)
				return
			}

			// Special handling for WebSocket connections
			// WebSocket connections include the Upgrade header with value 'websocket'
			if strings.ToLower(r.Header.Get("Upgrade")) == "websocket" {
				// For WebSocket connections, don't wrap the response writer
				next.ServeHTTP(w, r)

				// Log WebSocket connection after the handler finishes
				duration := time.Since(start)
				log.Printf(
					"%s %s %s WebSocket %s",
					r.RemoteAddr,
					r.Method,
					r.URL.Path,
					duration,
				)
				return
			}

			// Create a custom response writer to capture status code for regular HTTP requests
			rw := &responseWriter{w, http.StatusOK}

			// Process request
			next.ServeHTTP(rw, r)

			// Log request details
			duration := time.Since(start)
			log.Printf(
				"%s %s %s %d %s",
				r.RemoteAddr,
				r.Method,
				r.URL.Path,
				rw.statusCode,
				duration,
			)
		})
	}
}

// Recover returns a middleware that recovers from panics
func Recover() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Printf("PANIC: %v\n%s", err, debug.Stack())
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// CORS returns a middleware that handles CORS
func CORS(allowedOrigins []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get the Origin header
			origin := r.Header.Get("Origin")

			// Set default value to "*" for allowing all origins
			corsOrigin := "*"

			// If specific origins are provided, check if the request origin is allowed
			if len(allowedOrigins) > 0 && origin != "" {
				// Check for wildcard
				for _, allowed := range allowedOrigins {
					if allowed == "*" {
						corsOrigin = "*"
						break
					}
					if allowed == origin {
						corsOrigin = origin
						break
					}
				}
			}

			// Set CORS headers
			w.Header().Set("Access-Control-Allow-Origin", corsOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// responseWriter is a wrapper for http.ResponseWriter that captures the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code and passes it to the underlying ResponseWriter
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Hijack implements the http.Hijacker interface to allow WebSocket connections
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, errors.New("http.Hijacker interface is not supported by the underlying ResponseWriter")
}

// Flush implements the http.Flusher interface
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Push implements the http.Pusher interface for HTTP/2 support
func (rw *responseWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := rw.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}
