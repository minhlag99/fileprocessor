package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	ProviderGoogle = "google"
)

type UserSession struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Provider  string    `json:"provider"`
	Expiry    time.Time `json:"expiry"`
	CreatedAt time.Time `json:"createdAt"`
}

type User struct {
	ID        string            `json:"id"`
	Email     string            `json:"email"`
	Name      string            `json:"name"`
	AvatarURL string            `json:"avatarUrl"`
	Provider  string            `json:"provider"`
	Metadata  map[string]string `json:"metadata"`
}

type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type AuthManager struct {
	mu            sync.RWMutex
	users         map[string]*User
	sessions      map[string]*UserSession
	configs       map[string]*OAuthConfig
	oauthConfigs  map[string]*oauth2.Config
	userConfigs   map[string]map[string]map[string]string
	dataDir       string
	sessionExpiry time.Duration
	sessionSecret string
}

func NewAuthManager() *AuthManager {
	sessionSecret := generateSecureSecret(32)

	return &AuthManager{
		users:         make(map[string]*User),
		sessions:      make(map[string]*UserSession),
		configs:       make(map[string]*OAuthConfig),
		oauthConfigs:  make(map[string]*oauth2.Config),
		userConfigs:   make(map[string]map[string]map[string]string),
		dataDir:       "./data/auth",
		sessionExpiry: 24 * time.Hour,
		sessionSecret: sessionSecret,
	}
}

func generateSecureSecret(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_=+[]{}|;:,.<>?"
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		log.Printf("Warning: Could not generate secure random secret: %v", err)
		return fmt.Sprintf("%d.%s", time.Now().UnixNano(), base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano()))))
	}

	for i, b := range bytes {
		bytes[i] = chars[int(b)%len(chars)]
	}
	return string(bytes)
}

func (m *AuthManager) Init(googleClientID, googleClientSecret, redirectURL string) error {
	if err := os.MkdirAll(m.dataDir, 0755); err != nil {
		return err
	}

	if err := m.loadUsers(); err != nil {
		log.Printf("Failed to load users: %v", err)
	}

	if err := m.loadSessions(); err != nil {
		log.Printf("Failed to load sessions: %v", err)
	}

	if err := m.loadUserConfigs(); err != nil {
		log.Printf("Failed to load user configs: %v", err)
	}

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

	go m.cleanupSessions()

	return nil
}

func (m *AuthManager) GenerateLoginURL(provider string) (string, string, error) {
	m.mu.RLock()
	oauthConfig, ok := m.oauthConfigs[provider]
	m.mu.RUnlock()

	if !ok {
		return "", "", fmt.Errorf("provider %s not configured", provider)
	}

	state, err := generateRandomState()
	if err != nil {
		return "", "", err
	}

	url := oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)

	return url, state, nil
}

func (m *AuthManager) HandleCallback(r *http.Request) (*UserSession, error) {
	code := r.URL.Query().Get("code")
	if code == "" {
		return nil, errors.New("code not found in callback")
	}

	provider := r.URL.Query().Get("provider")
	if provider == "" {
		provider = ProviderGoogle
	}

	m.mu.RLock()
	oauthConfig, ok := m.oauthConfigs[provider]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("provider %s not configured", provider)
	}

	token, err := oauthConfig.Exchange(r.Context(), code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}

	userInfo, err := m.getUserInfo(r.Context(), provider, token)
	if err != nil {
		return nil, err
	}

	session, err := m.createSession(userInfo.ID, provider)
	if err != nil {
		return nil, err
	}

	return session, nil
}

func (m *AuthManager) LogOut(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, sessionID)

	return nil
}

func (m *AuthManager) GetSession(sessionID string) (*UserSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, errors.New("session not found")
	}

	return session, nil
}

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

func (m *AuthManager) SaveUserCloudConfig(userID, provider string, config map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.userConfigs[userID]; !ok {
		m.userConfigs[userID] = make(map[string]map[string]string)
	}

	m.userConfigs[userID][provider] = config

	return m.saveUserConfigs()
}

func (m *AuthManager) getUserInfo(ctx context.Context, provider string, token *oauth2.Token) (*User, error) {
	oauthConfig, ok := m.oauthConfigs[provider]
	if !ok {
		return nil, fmt.Errorf("provider %s not configured", provider)
	}

	client := oauthConfig.Client(ctx, token)

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

	m.mu.Lock()
	defer m.mu.Unlock()

	existingUser, ok := m.findUserByEmail(userInfo.Email, provider)
	if ok {
		existingUser.Name = userInfo.Name
		existingUser.AvatarURL = userInfo.AvatarURL
		if err := m.saveUsers(); err != nil {
			log.Printf("Failed to save users: %v", err)
		}
		return existingUser, nil
	}

	m.users[userInfo.ID] = userInfo
	if err := m.saveUsers(); err != nil {
		log.Printf("Failed to save users: %v", err)
	}

	return userInfo, nil
}

func (m *AuthManager) findUserByEmail(email, provider string) (*User, bool) {
	for _, user := range m.users {
		if user.Email == email && user.Provider == provider {
			return user, true
		}
	}
	return nil, false
}

func (m *AuthManager) createSession(userID, provider string) (*UserSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionID, err := generateRandomState()
	if err != nil {
		return nil, err
	}

	session := &UserSession{
		ID:        sessionID,
		UserID:    userID,
		Provider:  provider,
		Expiry:    time.Now().Add(m.sessionExpiry),
		CreatedAt: time.Now(),
	}

	m.sessions[sessionID] = session

	if err := m.saveSessions(); err != nil {
		log.Printf("Failed to save sessions: %v", err)
	}

	return session, nil
}

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

func (m *AuthManager) loadSessions() error {
	filePath := filepath.Join(m.dataDir, "sessions.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil
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

func (m *AuthManager) loadUsers() error {
	filePath := filepath.Join(m.dataDir, "users.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil
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

func (m *AuthManager) saveUserConfigs() error {
	data, err := json.MarshalIndent(m.userConfigs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal user configs: %w", err)
	}

	configPath := filepath.Join(m.dataDir, "user_configs.json")

	file, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open user configs file for writing: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("failed to write user configs data: %w", err)
	}

	return nil
}

func (m *AuthManager) loadUserConfigs() error {
	filePath := filepath.Join(m.dataDir, "user_configs.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil
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

func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func (m *AuthManager) generateAuthToken() (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}

	token := base64.URLEncoding.EncodeToString(tokenBytes)

	timestamp := time.Now().UnixNano()
	combined := fmt.Sprintf("%s.%d", token, timestamp)

	hash := sha256.Sum256([]byte(combined))
	finalToken := fmt.Sprintf("%s.%x", token, hash[:8])

	return finalToken, nil
}

func (m *AuthManager) generateCSRFToken(sessionID string) (string, error) {
	csrfBytes := make([]byte, 16)
	if _, err := rand.Read(csrfBytes); err != nil {
		return "", fmt.Errorf("failed to generate CSRF token: %w", err)
	}

	rawToken := base64.StdEncoding.EncodeToString(csrfBytes)

	h := hmac.New(sha256.New, []byte(m.sessionSecret))
	h.Write([]byte(sessionID))
	h.Write([]byte(rawToken))
	signature := h.Sum(nil)

	return fmt.Sprintf("%s.%s", rawToken, base64.URLEncoding.EncodeToString(signature[:10])), nil
}

func (m *AuthManager) validateCSRFToken(token, sessionID string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}

	rawToken := parts[0]
	signature := parts[1]

	h := hmac.New(sha256.New, []byte(m.sessionSecret))
	h.Write([]byte(sessionID))
	h.Write([]byte(rawToken))
	expectedSig := h.Sum(nil)

	signatureBytes, err := base64.URLEncoding.DecodeString(signature)
	if err != nil {
		return false
	}

	return hmac.Equal(signatureBytes, expectedSig[:len(signatureBytes)])
}

var DefaultAuthManager = NewAuthManager()

func Init(googleClientID, googleClientSecret, redirectURL string) error {
	return DefaultAuthManager.Init(googleClientID, googleClientSecret, redirectURL)
}
