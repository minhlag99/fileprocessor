// Package auth provides authentication and authorization features
package auth

import (
	"log"
	"net/http"
	"time"
)

// Middleware provides authentication middleware
type Middleware struct {
	authManager *AuthManager
}

// NewMiddleware creates a new authentication middleware
func NewMiddleware(manager *AuthManager) *Middleware {
	if manager == nil {
		manager = DefaultAuthManager
	}
	return &Middleware{
		authManager: manager,
	}
}

// RequireAuth ensures that a request is authenticated
func (m *Middleware) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check for session cookie
		cookie, err := r.Cookie("session_id")
		if err != nil {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}

		// Validate session
		session, err := m.authManager.GetSession(cookie.Value)
		if err != nil {
			http.Error(w, "Invalid session", http.StatusUnauthorized)
			return
		}

		// Check if session is expired using time.Now() instead of http.TimeProvider
		if session.Expiry.Before(time.Now()) {
			http.Error(w, "Session expired", http.StatusUnauthorized)
			return
		}

		// Add user and session info to request context
		ctx := WithSession(r.Context(), session)
		next(w, r.WithContext(ctx))
	}
}

// RequireAuthOptional allows anonymous or authenticated requests
func (m *Middleware) RequireAuthOptional(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check for session cookie
		cookie, err := r.Cookie("session_id")
		if err == nil {
			// Validate session
			session, err := m.authManager.GetSession(cookie.Value)
			if err == nil {
				// Check if session is expired using time.Now() instead of http.TimeProvider
				if !session.Expiry.Before(time.Now()) {
					// Add user and session info to request context
					ctx := WithSession(r.Context(), session)
					next(w, r.WithContext(ctx))
					return
				}
			}
		}

		// Continue without authentication
		next(w, r)
	}
}

// DefaultMiddleware is the default authentication middleware
var DefaultMiddleware = NewMiddleware(DefaultAuthManager)

// AuthMiddlewareHandler is middleware that checks if the user is authenticated
func AuthMiddlewareHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication for public endpoints
		if isPublicEndpoint(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Get the session ID from the cookie
		sessionID, ok := GetSessionCookie(r)
		if !ok {
			// No session cookie found
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}

		// Validate the session and get the user
		user, err := DefaultAuthManager.GetUserBySession(sessionID)
		if err != nil {
			// Session is invalid or expired
			log.Printf("Invalid session: %v", err)
			ClearSessionCookie(w)
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}

		// Add user and session to the request context
		session, _ := DefaultAuthManager.GetSession(sessionID)
		ctx := WithUser(r.Context(), user)
		ctx = WithSession(ctx, session)
		r = r.WithContext(ctx)

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}

// RequireAuth is middleware that requires authentication and redirects to login if not authenticated
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the session ID from the cookie
		sessionID, ok := GetSessionCookie(r)
		if !ok {
			// No session cookie found, redirect to login
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Validate the session and get the user
		user, err := DefaultAuthManager.GetUserBySession(sessionID)
		if err != nil {
			// Session is invalid or expired, redirect to login
			ClearSessionCookie(w)
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		// Add user and session to the request context
		session, _ := DefaultAuthManager.GetSession(sessionID)
		ctx := WithUser(r.Context(), user)
		ctx = WithSession(ctx, session)
		r = r.WithContext(ctx)

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}

// isPublicEndpoint determines if a path is a public endpoint that doesn't require authentication
func isPublicEndpoint(path string) bool {
	// Add your public endpoints here
	publicEndpoints := []string{
		"/",
		"/api/auth/login",
		"/api/auth/callback",
		"/health",
		"/favicon.ico",
		"/ui/",
		"/static/",
		"/images/",
	}

	// Static file extensions that don't require authentication
	publicExtensions := []string{
		".html", ".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".woff", ".woff2", ".ttf", ".eot",
	}

	// Check if the path is in the public endpoints
	for _, endpoint := range publicEndpoints {
		if path == endpoint || (len(endpoint) > 0 && endpoint[len(endpoint)-1] == '/' && len(path) > len(endpoint) && path[:len(endpoint)] == endpoint) {
			return true
		}
	}

	// Check if the path has a public extension
	for _, ext := range publicExtensions {
		if len(path) > len(ext) && path[len(path)-len(ext):] == ext {
			return true
		}
	}

	return false
}
