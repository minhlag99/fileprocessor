// Package auth provides authentication and authorization features
package auth

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Handler handles authentication requests
type Handler struct {
	authManager *AuthManager
}

// NewHandler creates a new authentication handler
func NewHandler(manager *AuthManager) *Handler {
	if manager == nil {
		manager = DefaultAuthManager
	}
	return &Handler{
		authManager: manager,
	}
}

// HandleLogin handles login requests
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	provider := r.URL.Query().Get("provider")
	if provider == "" {
		provider = ProviderGoogle // Default to Google
	}

	// Generate login URL
	url, state, err := h.authManager.GenerateLoginURL(provider)
	if err != nil {
		http.Error(w, "Failed to generate login URL: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Set state cookie for CSRF protection
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   int(time.Hour.Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	// Respond with login URL
	response := map[string]string{
		"url": url,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleCallback handles OAuth callback
func (h *Handler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify state for CSRF protection
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		http.Error(w, "State verification failed", http.StatusBadRequest)
		return
	}

	state := r.URL.Query().Get("state")
	if state != stateCookie.Value {
		http.Error(w, "State mismatch", http.StatusBadRequest)
		return
	}

	// Process callback
	session, err := h.authManager.HandleCallback(r)
	if err != nil {
		http.Error(w, "Authentication failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Clear state cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Path:     "/",
		Expires:  session.Expiry,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect to profile page
	http.Redirect(w, r, "/profile", http.StatusFound)
}

// HandleProfile handles user profile requests
func (h *Handler) HandleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get session cookie
	cookie, err := r.Cookie("session_id")
	if err != nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Get user from session
	user, err := h.authManager.GetUserBySession(cookie.Value)
	if err != nil {
		http.Error(w, "Invalid session", http.StatusUnauthorized)
		return
	}

	// Return user profile
	response := map[string]interface{}{
		"id":        user.ID,
		"name":      user.Name,
		"email":     user.Email,
		"avatarURL": user.AvatarURL,
		"provider":  user.Provider,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleLogout handles logout requests
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get session cookie
	cookie, err := r.Cookie("session_id")
	if err == nil {
		// Invalidate session
		h.authManager.LogOut(cookie.Value)
	}

	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:   "session_id",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	// Return success
	response := map[string]bool{
		"success": true,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleCloudConfig handles cloud configuration requests
func (h *Handler) HandleCloudConfig(w http.ResponseWriter, r *http.Request) {
	// Get session cookie
	cookie, err := r.Cookie("session_id")
	if err != nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Get user from session
	session, err := h.authManager.GetSession(cookie.Value)
	if err != nil {
		http.Error(w, "Invalid session", http.StatusUnauthorized)
		return
	}

	provider := r.URL.Query().Get("provider")
	if provider == "" {
		http.Error(w, "Provider is required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Get cloud config
		config, err := h.authManager.GetUserCloudConfig(session.UserID, provider)
		if err != nil {
			http.Error(w, "Failed to get cloud config: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Return cloud config
		response := map[string]interface{}{
			"provider": provider,
			"config":   config,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)

	case http.MethodPost:
		// Parse config from request
		var config map[string]string
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Save cloud config
		if err := h.authManager.SaveUserCloudConfig(session.UserID, provider, config); err != nil {
			http.Error(w, "Failed to save cloud config: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Return success
		response := map[string]bool{
			"success": true,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// DefaultHandler is the default authentication handler
var DefaultHandler = NewHandler(DefaultAuthManager)

// DefaultAuthHandler is the default authentication handler - alias for DefaultHandler for backwards compatibility
var DefaultAuthHandler = DefaultHandler

// LoginHandler handles OAuth login requests
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	// Get the provider from the query string
	provider := r.URL.Query().Get("provider")
	if provider == "" {
		provider = ProviderGoogle // Default to Google
	}

	// Generate a login URL
	loginURL, state, err := DefaultAuthManager.GenerateLoginURL(provider)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate login URL: %v", err), http.StatusInternalServerError)
		return
	}

	// Return the login URL as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"url":    loginURL,
		"state":  state,
		"status": "success",
	})
}

// CallbackHandler handles OAuth callback requests
func CallbackHandler(w http.ResponseWriter, r *http.Request) {
	// Process the callback
	session, err := DefaultAuthManager.HandleCallback(r)
	if err != nil {
		log.Printf("OAuth callback error: %v", err)
		http.Error(w, fmt.Sprintf("Failed to authenticate: %v", err), http.StatusInternalServerError)
		return
	}

	// Set the session cookie
	SetSessionCookie(w, session.ID)

	// Redirect back to the application
	redirectURL := r.URL.Query().Get("redirect_uri")
	if redirectURL == "" {
		redirectURL = "/" // Default to home page
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// LogoutHandler handles logout requests
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// Get the session ID from the cookie
	sessionID, ok := GetSessionCookie(r)
	if !ok {
		// No session to log out from
		http.Error(w, "Not authenticated", http.StatusBadRequest)
		return
	}

	// Invalidate the session
	if err := DefaultAuthManager.LogOut(sessionID); err != nil {
		log.Printf("Failed to logout: %v", err)
	}

	// Clear the session cookie
	ClearSessionCookie(w)

	// Return success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	})
}

// ProfileHandler returns the authenticated user's profile
func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	// Get the session ID from the cookie
	sessionID, ok := GetSessionCookie(r)
	if !ok {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	// Get the user from the session
	user, err := DefaultAuthManager.GetUserBySession(sessionID)
	if err != nil {
		// Session is invalid or expired
		ClearSessionCookie(w)
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	// Return the user profile
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":        user.ID,
		"name":      user.Name,
		"email":     user.Email,
		"avatarURL": user.AvatarURL,
		"provider":  user.Provider,
	})
}

// CloudConfigHandler manages user cloud storage configuration
func CloudConfigHandler(w http.ResponseWriter, r *http.Request) {
	// Get the session ID from the cookie
	sessionID, ok := GetSessionCookie(r)
	if !ok {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	// Get the user from the session
	user, err := DefaultAuthManager.GetUserBySession(sessionID)
	if err != nil {
		// Session is invalid or expired
		ClearSessionCookie(w)
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	// Get the provider from the query string
	provider := r.URL.Query().Get("provider")
	if provider == "" {
		http.Error(w, "Provider not specified", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodGet {
		// Get the cloud config for the user
		config, err := DefaultAuthManager.GetUserCloudConfig(user.ID, provider)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get cloud config: %v", err), http.StatusInternalServerError)
			return
		}

		// Return the cloud config
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"config":  config,
		})
	} else if r.Method == http.MethodPost {
		// Parse the request body
		var config map[string]string
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse request body: %v", err), http.StatusBadRequest)
			return
		}

		// Save the cloud config for the user
		if err := DefaultAuthManager.SaveUserCloudConfig(user.ID, provider, config); err != nil {
			http.Error(w, fmt.Sprintf("Failed to save cloud config: %v", err), http.StatusInternalServerError)
			return
		}

		// Return success
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
		})
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// RegisterHandlers registers the authentication handlers with the router
func RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/auth/login", LoginHandler)
	mux.HandleFunc("/api/auth/callback", CallbackHandler)
	mux.HandleFunc("/api/auth/logout", LogoutHandler)
	mux.HandleFunc("/api/auth/profile", ProfileHandler)
	mux.HandleFunc("/api/auth/cloud-config", CloudConfigHandler)
}
