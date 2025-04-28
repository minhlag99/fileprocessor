// Package middleware provides HTTP middleware functions
package middleware

import (
	"log"
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

			// Create a custom response writer to capture status code
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
func CORS(allowedOrigins string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set CORS headers
			origins := "*"
			if allowedOrigins != "" {
				origins = allowedOrigins
			}
			
			w.Header().Set("Access-Control-Allow-Origin", origins)
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