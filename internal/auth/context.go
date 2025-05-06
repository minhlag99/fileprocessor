// Package auth provides authentication and authorization features
package auth

import (
	"context"
	"net/http"
	"time"
)

type contextKey string

const (
	// UserContextKey is the key used to store the user in the context
	UserContextKey contextKey = "user"
	// SessionContextKey is the key used to store the session in the context
	SessionContextKey contextKey = "session"

	// SessionCookieName is the name of the session cookie
	SessionCookieName = "fileprocessor_session"
)

// WithUser stores the user in the context
func WithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, UserContextKey, user)
}

// UserFromContext retrieves the user from the context
func UserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(UserContextKey).(*User)
	return user, ok
}

// WithSession stores the session in the context
func WithSession(ctx context.Context, session *UserSession) context.Context {
	return context.WithValue(ctx, SessionContextKey, session)
}

// SessionFromContext retrieves the session from the context
func SessionFromContext(ctx context.Context) (*UserSession, bool) {
	session, ok := ctx.Value(SessionContextKey).(*UserSession)
	return session, ok
}

// SetSessionCookie sets the session cookie
func SetSessionCookie(w http.ResponseWriter, sessionID string) {
	// Create a new cookie
	cookie := &http.Cookie{
		Name:     SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   86400 * 7, // 7 days
	}

	// Set the cookie
	http.SetCookie(w, cookie)
}

// ClearSessionCookie clears the session cookie
func ClearSessionCookie(w http.ResponseWriter) {
	// Create an expired cookie
	cookie := &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1, // Delete the cookie
	}

	// Set the cookie
	http.SetCookie(w, cookie)
}

// GetSessionCookie gets the session ID from the cookie
func GetSessionCookie(r *http.Request) (string, bool) {
	// Get the cookie
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return "", false
	}

	return cookie.Value, true
}

// IsAuthenticated checks if the context has a valid session
func IsAuthenticated(ctx context.Context) bool {
	session, ok := SessionFromContext(ctx)
	if !ok {
		return false
	}

	// Check if session is expired
	return !session.Expiry.Before(time.Now())
}
