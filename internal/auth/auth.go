// Package auth provides authentication and authorization features
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	// ProviderGoogle is the name of the Google OAuth provider
	ProviderGoogle = "google"
)

// UserSession represents a user session
type UserSession struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Provider  string    `json:"provider"`
	Expiry    time.Time `json:"expiry"`
	CreatedAt time.Time `json:"createdAt"`
}

// User represents an authenticated user
type User struct {
	ID        string            `json:"id"`
	Email     string            `json:"email"`
	Name      string            `json:"name"`
	AvatarURL string            `json:"avatarUrl"`
	Provider  string            `json:"provider"`
	Metadata  map[string]string `json:"metadata"`
}

// OAuthConfig represents OAuth2 configuration
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// AuthManager manages authentication
type AuthManager struct {
	mu            sync.RWMutex
	users         map[string]*User        // Map of user ID to user
	sessions      map[string]*UserSession // Map of session ID to session
	configs       map[string]*OAuthConfig // Map of provider to OAuth config
	oauthConfigs  map[string]*oauth2.Config
	userConfigs   map[string]map[string]map[string]string // userId -> provider -> config
	dataDir       string
	sessionExpiry time.Duration
}

// NewAuthManager creates a new authentication manager
func NewAuthManager() *AuthManager {
	return &AuthManager{
		users:         make(map[string]*User),
		sessions:      make(map[string]*UserSession),
		configs:       make(map[string]*OAuthConfig),
		oauthConfigs:  make(map[string]*oauth2.Config),
		userConfigs:   make(map[string]map[string]map[string]string),
		dataDir:       "./data/auth",
		sessionExpiry: 24 * time.Hour,
	}
}

// Init initializes the authentication manager with OAuth providers
func (m *AuthManager) Init(googleClientID, googleClientSecret, redirectURL string) error {
	// Create data directory if it doesn't exist
	if err := os.MkdirAll(m.dataDir, 0755); err != nil {
		return err
	}

	// Load users from disk
	if err := m.loadUsers(); err != nil {
		log.Printf("Failed to load users: %v", err)
	}

	// Load sessions from disk
	if err := m.loadSessions(); err != nil {
		log.Printf("Failed to load sessions: %v", err)
	}

	// Load user configs from disk
	if err := m.loadUserConfigs(); err != nil {
		log.Printf("Failed to load user configs: %v", err)
	}

	// Configure OAuth providers
	if googleClientID != "" && googleClientSecret != "" {
		m.configs[ProviderGoogle] = &OAuthConfig{
			ClientID:     googleClientID,
			ClientSecret: googleClientSecret,
			RedirectURL:  redirectURL,
		}

		m.oauthConfigs[ProviderGoogle] = &oauth2.Config{
			ClientID:     googleClientID,
			ClientSecret: googleClientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"profile", "email"},
			Endpoint:     google.Endpoint,
		}
	}

	// Start a goroutine to remove expired sessions
	go m.cleanupSessions()

	return nil
}

// GenerateLoginURL generates a login URL for the specified provider
func (m *AuthManager) GenerateLoginURL(provider string) (string, string, error) {
	m.mu.RLock()
	oauthConfig, ok := m.oauthConfigs[provider]
	m.mu.RUnlock()

	if !ok {
		return "", "", fmt.Errorf("provider %s not configured", provider)
	}

	// Generate a random state
	state, err := generateRandomState()
	if err != nil {
		return "", "", err
	}

	// Generate the login URL
	url := oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)

	return url, state, nil
}

// HandleCallback processes an OAuth callback request
func (m *AuthManager) HandleCallback(r *http.Request) (*UserSession, error) {
	// Get the code and state from the request
	code := r.URL.Query().Get("code")
	if code == "" {
		return nil, errors.New("code not found in callback")
	}

	// Get the provider from the request
	provider := r.URL.Query().Get("provider")
	if provider == "" {
		provider = ProviderGoogle // Default to Google
	}

	// Find the OAuth config for the provider
	m.mu.RLock()
	oauthConfig, ok := m.oauthConfigs[provider]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("provider %s not configured", provider)
	}

	// Exchange the code for a token
	token, err := oauthConfig.Exchange(r.Context(), code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}

	// Get the user info for the user
	userInfo, err := m.getUserInfo(r.Context(), provider, token)
	if err != nil {
		return nil, err
	}

	// Create a session for the user
	session, err := m.createSession(userInfo.ID, provider)
	if err != nil {
		return nil, err
	}

	return session, nil
}

// LogOut invalidates a session
func (m *AuthManager) LogOut(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Delete the session
	delete(m.sessions, sessionID)

	return nil
}

// GetSession retrieves a session by ID
func (m *AuthManager) GetSession(sessionID string) (*UserSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, errors.New("session not found")
	}

	return session, nil
}

// GetUserBySession retrieves a user by session ID
func (m *AuthManager) GetUserBySession(sessionID string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, errors.New("session not found")
	}

	if session.Expiry.Before(time.Now()) {
		return nil, errors.New("session expired")
	}

	user, ok := m.users[session.UserID]
	if !ok {
		return nil, errors.New("user not found")
	}

	return user, nil
}

// GetUserCloudConfig retrieves a user's cloud configuration
func (m *AuthManager) GetUserCloudConfig(userID, provider string) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userConfigs, ok := m.userConfigs[userID]
	if !ok {
		return map[string]string{}, nil
	}

	config, ok := userConfigs[provider]
	if !ok {
		return map[string]string{}, nil
	}

	return config, nil
}

// SaveUserCloudConfig saves a user's cloud configuration
func (m *AuthManager) SaveUserCloudConfig(userID, provider string, config map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure we have a map for this user
	if _, ok := m.userConfigs[userID]; !ok {
		m.userConfigs[userID] = make(map[string]map[string]string)
	}

	// Save the config
	m.userConfigs[userID][provider] = config

	// Save to disk
	return m.saveUserConfigs()
}

// getUserInfo retrieves user information from the OAuth provider
func (m *AuthManager) getUserInfo(ctx context.Context, provider string, token *oauth2.Token) (*User, error) {
	// Get the OAuth config for the provider
	oauthConfig, ok := m.oauthConfigs[provider]
	if !ok {
		return nil, fmt.Errorf("provider %s not configured", provider)
	}

	// Create an HTTP client with the token
	client := oauthConfig.Client(ctx, token)

	// Get the user info from the provider
	var userInfo *User
	var err error

	switch provider {
	case ProviderGoogle:
		userInfo, err = getGoogleUserInfo(client)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	if err != nil {
		return nil, err
	}

	// Save or update the user
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if the user already exists
	existingUser, ok := m.findUserByEmail(userInfo.Email, provider)
	if ok {
		// Update the existing user
		existingUser.Name = userInfo.Name
		existingUser.AvatarURL = userInfo.AvatarURL
		// Save the users
		if err := m.saveUsers(); err != nil {
			log.Printf("Failed to save users: %v", err)
		}
		return existingUser, nil
	}

	// Save the new user
	m.users[userInfo.ID] = userInfo
	// Save the users
	if err := m.saveUsers(); err != nil {
		log.Printf("Failed to save users: %v", err)
	}

	return userInfo, nil
}

// findUserByEmail finds a user by email and provider
func (m *AuthManager) findUserByEmail(email, provider string) (*User, bool) {
	for _, user := range m.users {
		if user.Email == email && user.Provider == provider {
			return user, true
		}
	}
	return nil, false
}

// createSession creates a new session for a user
func (m *AuthManager) createSession(userID, provider string) (*UserSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate a session ID
	sessionID, err := generateRandomState()
	if err != nil {
		return nil, err
	}

	// Create the session
	session := &UserSession{
		ID:        sessionID,
		UserID:    userID,
		Provider:  provider,
		Expiry:    time.Now().Add(m.sessionExpiry),
		CreatedAt: time.Now(),
	}

	// Store the session
	m.sessions[sessionID] = session

	// Save the sessions
	if err := m.saveSessions(); err != nil {
		log.Printf("Failed to save sessions: %v", err)
	}

	return session, nil
}

// cleanupSessions removes expired sessions
func (m *AuthManager) cleanupSessions() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for id, session := range m.sessions {
			if session.Expiry.Before(now) {
				delete(m.sessions, id)
			}
		}
		m.saveUsers()
		m.mu.Unlock()
	}
}

// saveSessions saves all sessions to disk
func (m *AuthManager) saveSessions() error {
	data, err := json.MarshalIndent(m.sessions, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal sessions: %w", err)
	}

	if err := os.WriteFile(filepath.Join(m.dataDir, "sessions.json"), data, 0644); err != nil {
		return fmt.Errorf("failed to write sessions file: %w", err)
	}

	return nil
}

// loadSessions loads all sessions from disk
func (m *AuthManager) loadSessions() error {
	filePath := filepath.Join(m.dataDir, "sessions.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // File doesn't exist, which is fine
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read sessions file: %w", err)
	}

	if err := json.Unmarshal(data, &m.sessions); err != nil {
		return fmt.Errorf("failed to unmarshal sessions: %w", err)
	}

	return nil
}

// saveUsers saves all users to disk
func (m *AuthManager) saveUsers() error {
	data, err := json.MarshalIndent(m.users, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal users: %w", err)
	}

	if err := os.WriteFile(filepath.Join(m.dataDir, "users.json"), data, 0644); err != nil {
		return fmt.Errorf("failed to write users file: %w", err)
	}

	return nil
}

// loadUsers loads all users from disk
func (m *AuthManager) loadUsers() error {
	filePath := filepath.Join(m.dataDir, "users.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // File doesn't exist, which is fine
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read users file: %w", err)
	}

	if err := json.Unmarshal(data, &m.users); err != nil {
		return fmt.Errorf("failed to unmarshal users: %w", err)
	}

	return nil
}

// saveUserConfigs saves all user configs to disk
func (m *AuthManager) saveUserConfigs() error {
	data, err := json.MarshalIndent(m.userConfigs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal user configs: %w", err)
	}

	if err := os.WriteFile(filepath.Join(m.dataDir, "user_configs.json"), data, 0644); err != nil {
		return fmt.Errorf("failed to write user configs file: %w", err)
	}

	return nil
}

// loadUserConfigs loads all user configs from disk
func (m *AuthManager) loadUserConfigs() error {
	filePath := filepath.Join(m.dataDir, "user_configs.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // File doesn't exist, which is fine
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read user configs file: %w", err)
	}

	if err := json.Unmarshal(data, &m.userConfigs); err != nil {
		return fmt.Errorf("failed to unmarshal user configs: %w", err)
	}

	return nil
}

// getGoogleUserInfo retrieves user information from Google
func getGoogleUserInfo(client *http.Client) (*User, error) {
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	var userInfo struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		EmailVerified bool   `json:"email_verified"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	if !userInfo.EmailVerified {
		return nil, errors.New("email not verified")
	}

	return &User{
		ID:        userInfo.Sub,
		Email:     userInfo.Email,
		Name:      userInfo.Name,
		AvatarURL: userInfo.Picture,
		Provider:  ProviderGoogle,
		Metadata:  make(map[string]string),
	}, nil
}

// generateRandomState generates a random state string
func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// DefaultAuthManager is the default authentication manager
var DefaultAuthManager = NewAuthManager()

// Init initializes the default authentication manager
func Init(googleClientID, googleClientSecret, redirectURL string) error {
	return DefaultAuthManager.Init(googleClientID, googleClientSecret, redirectURL)
}
