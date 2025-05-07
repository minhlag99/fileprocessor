package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/example/fileprocessor/internal/auth"
	"github.com/example/fileprocessor/internal/config"
)

// Set conservative buffer sizes and timeouts for security
var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		// Implement proper origin checking based on config
		origin := r.Header.Get("Origin")
		if origin == "" {
			// No origin header, check if it's a same-origin request
			return r.Header.Get("Sec-WebSocket-Version") != ""
		}

		// Check if the origin is in the allowed list from configuration
		allowedOrigins := config.AppConfig.Server.AllowedOrigins

		// If AllowedOrigins contains "*", allow all origins
		if containsWildcard(allowedOrigins) {
			return true
		}

		// Otherwise, check if origin is in the allowed list
		for _, allowed := range allowedOrigins {
			if origin == allowed {
				return true
			}
		}

		log.Printf("Rejected WebSocket connection from origin: %s", origin)
		return false
	},
	HandshakeTimeout: 10 * time.Second, // Limit time for handshake to prevent DoS
}

// Helper function to check if a slice contains the wildcard character
func containsWildcard(slice []string) bool {
	for _, item := range slice {
		if item == "*" {
			return true
		}
	}
	return false
}

// ClientMessage represents a message from a client
type ClientMessage struct {
	Type      string          `json:"type"`
	TaskID    string          `json:"taskId,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
	Timestamp int64           `json:"timestamp,omitempty"` // For tracking message age
	Nonce     string          `json:"nonce,omitempty"`     // For preventing replay attacks
}

// ServerMessage represents a message to a client
type ServerMessage struct {
	Type      string      `json:"type"`
	TaskID    string      `json:"taskId,omitempty"`
	Content   interface{} `json:"content,omitempty"`
	Timestamp int64       `json:"timestamp"`
	RequestID string      `json:"requestId,omitempty"` // For correlating responses with requests
}

// Client represents a connected websocket client
type Client struct {
	conn          *websocket.Conn
	send          chan ServerMessage
	clientID      string
	userID        string // Authenticated user ID if available
	tasks         []string
	hub           *WebSocketHub
	lastPing      time.Time
	closeMu       sync.Mutex
	isClosed      bool
	messageCount  int                    // Count messages to detect flooding
	lastRateReset time.Time              // For rate limiting
	connectedAt   time.Time              // When the client connected
	authenticated bool                   // Whether the client has authenticated
	metadata      map[string]interface{} // Additional client metadata
}

// WebSocketHub manages all connected clients
type WebSocketHub struct {
	// Client management
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client

	// Message broadcasting
	broadcast chan ServerMessage

	// Task subscription management
	tasks map[string][]string // taskID -> clientIDs

	// Connection statistics
	stats struct {
		totalConnections    int
		activeConnections   int
		messagesReceived    int
		messagesSent        int
		connectionsRejected int
		lastStatsReset      time.Time
	}

	// Concurrency control
	mu sync.RWMutex

	// Shutdown control
	shutdown   chan struct{}
	isShutdown bool
}

// NewWebSocketHub creates a new websocket hub
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan ServerMessage),
		tasks:      make(map[string][]string),
		shutdown:   make(chan struct{}),
		isShutdown: false,
	}
}

// Run runs the websocket hub in a goroutine
func (h *WebSocketHub) Run() {
	go func() {
		for {
			select {
			case <-h.shutdown:
				// Close all client connections cleanly
				h.mu.Lock()
				for _, client := range h.clients {
					// Send close message
					select {
					case client.send <- ServerMessage{
						Type:      "shutdown",
						Content:   map[string]string{"message": "Server shutting down"},
						Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
					}:
					default:
						// Skip if client's send buffer is full
					}

					// Close the connection after a short delay to allow the message to be sent
					go func(c *Client) {
						time.Sleep(200 * time.Millisecond)
						c.closeMu.Lock()
						if !c.isClosed {
							c.conn.WriteMessage(websocket.CloseMessage,
								websocket.FormatCloseMessage(websocket.CloseGoingAway, "Server shutting down"))
							c.conn.Close()
							c.isClosed = true
						}
						c.closeMu.Unlock()
					}(client)
				}
				h.mu.Unlock()
				return

			case client := <-h.register:
				// Check if hub is shutting down
				if h.isShutdown {
					client.conn.Close()
					continue
				}

				// Check for rate limiting on connections
				clientIP := strings.Split(client.clientID, "-")[0]
				if h.isRateLimited(clientIP) {
					client.conn.WriteMessage(websocket.CloseMessage,
						websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "Too many connections"))
					client.conn.Close()
					continue
				}

				h.mu.Lock()
				h.clients[client.clientID] = client
				h.stats.totalConnections++
				h.stats.activeConnections++
				h.mu.Unlock()

				// Send welcome message with server time for syncing
				client.send <- ServerMessage{
					Type: "connected",
					Content: map[string]interface{}{
						"message":        "Connected to file processing server",
						"serverTime":     time.Now().UnixNano() / int64(time.Millisecond),
						"maxMessageSize": 512 * 1024, // 512KB
					},
					Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
				}

			case client := <-h.unregister:
				h.mu.Lock()
				if _, ok := h.clients[client.clientID]; ok {
					delete(h.clients, client.clientID)
					h.stats.activeConnections--
					close(client.send)

					// Clean up task subscriptions
					for _, taskID := range client.tasks {
						if subscribers, exists := h.tasks[taskID]; exists {
							newSubscribers := make([]string, 0)
							for _, id := range subscribers {
								if id != client.clientID {
									newSubscribers = append(newSubscribers, id)
								}
							}

							if len(newSubscribers) > 0 {
								h.tasks[taskID] = newSubscribers
							} else {
								delete(h.tasks, taskID)
							}
						}
					}
				}
				h.mu.Unlock()

			case message := <-h.broadcast:
				// Set timestamp if not already set
				if message.Timestamp == 0 {
					message.Timestamp = time.Now().UnixNano() / int64(time.Millisecond)
				}

				// Check if this is a task-specific message
				if message.TaskID != "" {
					h.mu.RLock()
					subscribers, exists := h.tasks[message.TaskID]
					h.mu.RUnlock()

					if exists {
						for _, clientID := range subscribers {
							h.mu.RLock()
							client, ok := h.clients[clientID]
							h.mu.RUnlock()

							if ok {
								select {
								case client.send <- message:
									h.stats.messagesSent++
								default:
									// Failed to send, client might be slow or disconnected
									h.unregister <- client
								}
							}
						}
					}
				} else {
					// Broadcast to all clients
					h.mu.RLock()
					for _, client := range h.clients {
						select {
						case client.send <- message:
							h.stats.messagesSent++
						default:
							// Failed to send, client might be slow or disconnected
							h.unregister <- client
						}
					}
					h.mu.RUnlock()
				}
			}
		}
	}()

	// Start housekeeping goroutines
	go h.checkInactiveConnections()
	go h.resetStats()
}

// isRateLimited checks if an IP is connecting too frequently
func (h *WebSocketHub) isRateLimited(clientIP string) bool {
	// Example implementation - replace with a proper rate limiter in production
	h.mu.RLock()
	defer h.mu.RUnlock()

	count := 0
	now := time.Now()

	// Count connections from same IP in the last minute
	for _, client := range h.clients {
		if strings.Split(client.clientID, "-")[0] == clientIP &&
			now.Sub(client.connectedAt) < time.Minute {
			count++
		}
	}

	// Limit to 10 connections per IP per minute
	return count >= 10
}

// resetStats periodically resets statistics
func (h *WebSocketHub) resetStats() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-h.shutdown:
			return
		case <-ticker.C:
			h.mu.Lock()
			h.stats.lastStatsReset = time.Now()
			h.stats.messagesReceived = 0
			h.stats.messagesSent = 0
			h.stats.connectionsRejected = 0
			h.mu.Unlock()
		}
	}
}

// Subscribe subscribes a client to updates for a specific task
func (h *WebSocketHub) Subscribe(clientID, taskID string) {
	if taskID == "" || clientID == "" {
		return // Skip invalid input
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Add client to task subscribers
	if _, exists := h.tasks[taskID]; !exists {
		h.tasks[taskID] = make([]string, 0)
	}

	// Check if already subscribed
	for _, id := h.tasks[taskID] {
		if id == clientID {
			return
		}
	}

	h.tasks[taskID] = append(h.tasks[taskID], clientID)

	// Update client's task list
	if client, ok := h.clients[clientID]; ok {
		client.tasks = append(client.tasks, taskID)
	}
}

// Unsubscribe unsubscribes a client from updates for a specific task
func (h *WebSocketHub) Unsubscribe(clientID, taskID string) {
	if taskID == "" || clientID == "" {
		return // Skip invalid input
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if subscribers, exists := h.tasks[taskID]; exists {
		newSubscribers := make([]string, 0)
		for _, id := range subscribers {
			if id != clientID {
				newSubscribers = append(newSubscribers, id)
			}
		}

		if len(newSubscribers) > 0 {
			h.tasks[taskID] = newSubscribers
		} else {
			delete(h.tasks, taskID)
		}
	}

	// Update client's task list
	if client, ok := h.clients[clientID]; ok {
		newTasks := make([]string, 0)
		for _, id := range client.tasks {
			if id != taskID {
				newTasks = append(newTasks, id)
			}
		}
		client.tasks = newTasks
	}
}

// SendTaskUpdate sends an update about a specific task
func (h *WebSocketHub) SendTaskUpdate(taskID string, updateType string, content interface{}) {
	if taskID == "" || updateType == "" {
		log.Printf("Warning: Attempted to send task update with empty taskID or updateType")
		return
	}

	h.broadcast <- ServerMessage{
		Type:      updateType,
		TaskID:    taskID,
		Content:   content,
		Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
		RequestID: generateShortID(),
	}
}

// Broadcast sends a message to all connected clients
func (h *WebSocketHub) Broadcast(messageType string, content interface{}) {
	if messageType == "" {
		log.Printf("Warning: Attempted to broadcast with empty messageType")
		return
	}

	h.broadcast <- ServerMessage{
		Type:      messageType,
		Content:   content,
		Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
		RequestID: generateShortID(),
	}
}

// Shutdown gracefully shuts down the hub
func (h *WebSocketHub) Shutdown() {
	h.mu.Lock()
	if !h.isShutdown {
		h.isShutdown = true
		close(h.shutdown)
	}
	h.mu.Unlock()
}

// GetStats returns current WebSocket statistics
func (h *WebSocketHub) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return map[string]interface{}{
		"activeConnections":   h.stats.activeConnections,
		"totalConnections":    h.stats.totalConnections,
		"messagesReceived":    h.stats.messagesReceived,
		"messagesSent":        h.stats.messagesSent,
		"connectionsRejected": h.stats.connectionsRejected,
		"lastStatsReset":      h.stats.lastStatsReset,
		"taskCount":           len(h.tasks),
	}
}

// checkInactiveConnections periodically checks for inactive connections
func (h *WebSocketHub) checkInactiveConnections() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.shutdown:
			return
		case <-ticker.C:
			now := time.Now()
			timeout := 2 * time.Minute

			h.mu.RLock()
			clientsToCheck := make([]*Client, 0, len(h.clients))
			for _, client := range h.clients {
				clientsToCheck = append(clientsToCheck, client)
			}
			h.mu.RUnlock()

			// Check clients outside of the lock to avoid blocking
			for _, client := range clientsToCheck {
				if now.Sub(client.lastPing) > timeout {
					h.unregister <- client
				}
			}
		}
	}
}

// ServeWs handles websocket requests from clients
func ServeWs(hub *WebSocketHub, w http.ResponseWriter, r *http.Request) {
	// Check if hub is shutting down
	if hub.isShutdown {
		http.Error(w, "Server is shutting down", http.StatusServiceUnavailable)
		return
	}

	// Extract auth token if present
	authToken := r.URL.Query().Get("token")

	// Check rate limiting by IP
	clientIP := getClientIP(r)
	if hub.isRateLimited(clientIP) {
		hub.stats.connectionsRejected++
		http.Error(w, "Too many connections", http.StatusTooManyRequests)
		return
	}

	// Upgrade connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading connection: %v", err)
		return
	}

	// Generate a unique client ID with a hash component for security
	clientID := generateClientID(r)

	// Create the client
	client := &Client{
		conn:          conn,
		send:          make(chan ServerMessage, 256), // Buffer for 256 messages
		clientID:      clientID,
		tasks:         make([]string, 0),
		hub:           hub,
		lastPing:      time.Now(),
		isClosed:      false,
		messageCount:  0,
		lastRateReset: time.Now(),
		connectedAt:   time.Now(),
		authenticated: false, // Default to unauthenticated
		metadata:      make(map[string]interface{}),
	}

	// Set client metadata
	client.metadata["userAgent"] = r.UserAgent()
	client.metadata["remoteAddr"] = clientIP

	// Check authentication if token is provided
	if authToken != "" && config.AppConfig.Features.EnableAuth {
		// Try to authenticate with the auth system
		userID := validateAuthToken(authToken)
		if userID != "" {
			client.authenticated = true
			client.userID = userID
			log.Printf("WebSocket client authenticated with user ID: %s", userID)

			// Add authentication information to client metadata
			client.metadata["authenticated"] = true
			client.metadata["authMethod"] = "token"
		} else {
			// Invalid token, but still allow connection as unauthenticated
			log.Printf("WebSocket client provided invalid authentication token")
		}
	}

	// Register the client
	hub.register <- client

	// Start goroutines for reading and writing
	go client.readPump()
	go client.writePump()
}

// readPump pumps messages from the websocket to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.closeMu.Lock()
		c.isClosed = true
		c.conn.Close()
		c.closeMu.Unlock()
	}()

	c.conn.SetReadLimit(512 * 1024) // 512KB max message size for security
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		c.lastPing = time.Now()
		return nil
	})

	// Rate limiting setup
	messageRateLimit := 60 // Messages per minute
	messageRateWindow := time.Minute

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNoStatusReceived) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Rate limiting check
		if c.messageCount == 0 {
			c.lastRateReset = time.Now()
		}
		c.messageCount++

		// Reset counter after window elapses
		if time.Since(c.lastRateReset) > messageRateWindow {
			c.messageCount = 1
			c.lastRateReset = time.Now()
		}

		// Check if rate limit exceeded
		if c.messageCount > messageRateLimit {
			c.send <- ServerMessage{
				Type:      "error",
				Content:   map[string]string{"error": "Rate limit exceeded"},
				Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
			}
			continue
		}

		// Parse client message with a size limit for security
		if len(message) > 32*1024 {
			c.send <- ServerMessage{
				Type:      "error",
				Content:   map[string]string{"error": "Message too large"},
				Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
			}
			continue
		}

		var clientMsg ClientMessage
		if err := json.Unmarshal(message, &clientMsg); err != nil {
			c.send <- ServerMessage{
				Type:      "error",
				Content:   map[string]string{"error": "Invalid message format"},
				Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
			}
			continue
		}

		// Update stats
		c.hub.mu.Lock()
		c.hub.stats.messagesReceived++
		c.hub.mu.Unlock()

		// Validate message timestamp to prevent replay attacks (within last 5 minutes)
		if clientMsg.Timestamp > 0 {
			now := time.Now().UnixNano() / int64(time.Millisecond)
			if now-clientMsg.Timestamp > 5*60*1000 || clientMsg.Timestamp > now+60*1000 {
				c.send <- ServerMessage{
					Type:      "error",
					Content:   map[string]string{"error": "Message timestamp out of acceptable range"},
					Timestamp: now,
				}
				continue
			}
		}

		// Handle message based on type
		switch clientMsg.Type {
		case "subscribe":
			if clientMsg.TaskID != "" {
				c.hub.Subscribe(c.clientID, clientMsg.TaskID)
				c.send <- ServerMessage{
					Type:      "subscribed",
					TaskID:    clientMsg.TaskID,
					Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
					RequestID: generateShortID(),
				}
			}

		case "unsubscribe":
			if clientMsg.TaskID != "" {
				c.hub.Unsubscribe(c.clientID, clientMsg.TaskID)
				c.send <- ServerMessage{
					Type:      "unsubscribed",
					TaskID:    clientMsg.TaskID,
					Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
					RequestID: generateShortID(),
				}
			}

		case "ping":
			c.lastPing = time.Now()
			c.send <- ServerMessage{
				Type:      "pong",
				Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
				RequestID: generateShortID(),
			}

		case "authenticate":
			// Handle in-band authentication with token
			var authRequest struct {
				Token string `json:"token"`
			}

			if err := json.Unmarshal(clientMsg.Content, &authRequest); err == nil && authRequest.Token != "" {
				// Validate the token using the auth system
				userID := validateAuthToken(authRequest.Token)
				success := userID != ""

				if success {
					c.authenticated = true
					c.userID = userID
					c.metadata["authenticated"] = true
					c.metadata["authMethod"] = "message"

					// Try to get user info for the response
					var userName string
					var userEmail string

					user, err := auth.DefaultAuthManager.GetUserBySession(authRequest.Token)
					if err == nil && user != nil {
						userName = user.Name
						userEmail = user.Email
					}

					c.send <- ServerMessage{
						Type: "authenticated",
						Content: map[string]interface{}{
							"success": true,
							"userId":  userID,
							"name":    userName,
							"email":   userEmail,
						},
						Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
						RequestID: generateShortID(),
					}
				} else {
					c.send <- ServerMessage{
						Type: "authenticated",
						Content: map[string]interface{}{
							"success": false,
							"error":   "Invalid or expired authentication token",
						},
						Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
						RequestID: generateShortID(),
					}
				}
			} else {
				c.send <- ServerMessage{
					Type: "authenticated",
					Content: map[string]interface{}{
						"success": false,
						"error":   "Invalid authentication request format",
					},
					Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
					RequestID: generateShortID(),
				}
			}

		default:
			// Unknown message type
			c.send <- ServerMessage{
				Type:      "error",
				Content:   map[string]string{"error": "Unknown message type"},
				Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
				RequestID: generateShortID(),
			}
		}
	}
}

// writePump pumps messages from the hub to the websocket
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.closeMu.Lock()
		if !c.isClosed {
			c.conn.Close()
			c.isClosed = true
		}
		c.closeMu.Unlock()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Channel closed, close the connection
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Encode and send the message
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			jsonData, err := json.Marshal(message)
			if err != nil {
				log.Printf("Error marshaling message: %v", err)
				w.Close()
				continue
			}

			if _, err := w.Write(jsonData); err != nil {
				return
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			// Send ping
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// DefaultWebSocketHub is the default websocket hub
var DefaultWebSocketHub = NewWebSocketHub()

// Init initializes and starts the default websocket hub
func init() {
	DefaultWebSocketHub.Run()
}

// Helper function to get client IP from request
func getClientIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	// Get IP from RemoteAddr
	ip := r.RemoteAddr
	if comma := strings.LastIndex(ip, ":"); comma != -1 {
		ip = ip[:comma]
	}
	return ip
}

// Generate a short random ID for request correlation
func generateShortID() string {
	bytes := make([]byte, 6)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// Generate a secure client ID
func generateClientID(r *http.Request) string {
	// Start with IP and timestamp
	baseInfo := fmt.Sprintf("%s-%d", getClientIP(r), time.Now().UnixNano())

	// Add a random component
	randBytes := make([]byte, 8)
	rand.Read(randBytes)

	// Hash everything together
	hash := sha256.Sum256([]byte(baseInfo + string(randBytes) + r.UserAgent()))

	return fmt.Sprintf("%s-%s", getClientIP(r), base64.RawURLEncoding.EncodeToString(hash[:8]))
}

// validateAuthToken validates an authentication token and returns the user ID if valid
func validateAuthToken(token string) string {
	// Return empty string if token is empty
	if token == "" {
		return ""
	}

	// Attempt to get user from the auth system
	user, err := auth.DefaultAuthManager.GetUserBySession(token)
	if err != nil {
		log.Printf("Auth token validation failed: %v", err)
		return ""
	}

	// Return the user ID if token is valid
	return user.ID
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
