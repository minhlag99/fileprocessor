package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/example/fileprocessor/internal/models"
)

// LANTransferHandler handles peer-to-peer file transfers over the local network
type LANTransferHandler struct {
	// Map of active transfer sessions by sessionID
	sessions   map[string]*TransferSession
	sessionsMu sync.RWMutex

	// Discovery service for finding peers on the LAN
	discoveryService *DiscoveryService

	// Security token for transfer verification
	securityToken string
}

// TransferSession represents an active file transfer session
type TransferSession struct {
	SessionID   string
	Files       []*models.File
	SenderID    string
	ReceiverID  string
	Status      string // "pending", "accepted", "rejected", "transferring", "completed", "failed"
	CreatedAt   time.Time
	CompletedAt time.Time
	Progress    int // 0-100
	Error       string
	Token       string // Security token for verification
}

// DiscoveryService handles discovery of peers on the local network
type DiscoveryService struct {
	// Map of active peers by peerID
	peers   map[string]*PeerInfo
	peersMu sync.RWMutex

	// Broadcast address and port for discovery
	broadcastAddr *net.UDPAddr
	listenAddr    *net.UDPAddr

	// UDP connection for discovery broadcasts
	conn *net.UDPConn

	// Channel to signal shutdown
	quit chan struct{}

	// Whether the service is running
	running bool
}

// PeerInfo contains information about a peer on the LAN
type PeerInfo struct {
	PeerID     string    `json:"peerId"`
	Name       string    `json:"name"`
	IP         string    `json:"ip"`
	Port       int       `json:"port"`
	LastSeen   time.Time `json:"lastSeen"`
	DeviceType string    `json:"deviceType"`
}

// NewLANTransferHandler creates a new LAN transfer handler
func NewLANTransferHandler() (*LANTransferHandler, error) {
	// Create the discovery service
	discoveryService, err := NewDiscoveryService()
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery service: %w", err)
	}

	// Generate a secure random token for this instance
	token, err := generateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate security token: %w", err)
	}

	return &LANTransferHandler{
		sessions:         make(map[string]*TransferSession),
		discoveryService: discoveryService,
		securityToken:    token,
	}, nil
}

// generateSecureToken creates a cryptographically secure random token
func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Start starts the LAN transfer handler
func (h *LANTransferHandler) Start() error {
	// Start the discovery service
	if err := h.discoveryService.Start(); err != nil {
		return fmt.Errorf("failed to start discovery service: %w", err)
	}

	// Start a goroutine to clean up old sessions
	go h.cleanupOldSessions()

	log.Println("LAN transfer service started")
	return nil
}

// cleanupOldSessions periodically cleans up old sessions
func (h *LANTransferHandler) cleanupOldSessions() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		h.sessionsMu.Lock()
		now := time.Now()
		for id, session := range h.sessions {
			// Remove sessions older than 24 hours or completed/failed sessions older than 1 hour
			if now.Sub(session.CreatedAt) > 24*time.Hour ||
				((session.Status == "completed" || session.Status == "failed") &&
					now.Sub(session.CompletedAt) > 1*time.Hour) {
				delete(h.sessions, id)
			}
		}
		h.sessionsMu.Unlock()
	}
}

// Stop stops the LAN transfer handler
func (h *LANTransferHandler) Stop() {
	// Stop the discovery service
	h.discoveryService.Stop()

	log.Println("LAN transfer service stopped")
}

// HandleDiscoverPeers handles requests to discover peers on the LAN
func (h *LANTransferHandler) HandleDiscoverPeers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if discovery service is running
	if h.discoveryService == nil || !h.discoveryService.running {
		sendJSONError(w, "LAN discovery service is not available", http.StatusServiceUnavailable)
		return
	}

	// Get all currently discovered peers
	peers := h.discoveryService.GetPeers()

	// Filter out stale peers
	activePeers := make([]*PeerInfo, 0)
	for _, peer := range peers {
		// Only include peers seen in the last 2 minutes
		if time.Since(peer.LastSeen) < 2*time.Minute {
			// Don't expose internal IP addresses to the client
			sanitizedPeer := *peer
			sanitizedPeer.IP = "LAN" // Replace IP with generic indicator
			activePeers = append(activePeers, &sanitizedPeer)
		}
	}

	// Send response
	response := models.APIResponse{
		Success: true,
		Data:    activePeers,
	}

	sendJSONResponse(w, response, http.StatusOK)
}

// HandleInitiateTransfer handles requests to initiate a file transfer
func (h *LANTransferHandler) HandleInitiateTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Rate limiting - check if the client is making too many requests
	clientIP := getClientIP(r)
	if isRateLimited(clientIP, "initiate_transfer", 10) { // Max 10 transfers per minute
		sendJSONError(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
		return
	}

	// Parse the request
	var request struct {
		SenderID   string         `json:"senderId"`
		ReceiverID string         `json:"receiverId"`
		Files      []*models.File `json:"files"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		sendJSONError(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate the request
	if request.SenderID == "" || request.ReceiverID == "" || len(request.Files) == 0 {
		sendJSONError(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Validate file sizes
	var totalSize int64
	for _, file := range request.Files {
		if file.Size <= 0 || file.Size > 1024*1024*1024 { // 1GB max per file
			sendJSONError(w, "Invalid file size", http.StatusBadRequest)
			return
		}
		totalSize += file.Size
	}

	if totalSize > 10*1024*1024*1024 { // 10GB max total
		sendJSONError(w, "Total file size too large", http.StatusBadRequest)
		return
	}

	// Check if the receiver exists
	receiverInfo := h.discoveryService.GetPeer(request.ReceiverID)
	if receiverInfo == nil {
		sendJSONError(w, "Receiver not found on the LAN", http.StatusNotFound)
		return
	}

	// Check if the receiver is still active (seen in the last 5 minutes)
	if time.Since(receiverInfo.LastSeen) > 5*time.Minute {
		sendJSONError(w, "Receiver not active on the LAN", http.StatusBadRequest)
		return
	}

	// Generate a secure session ID and token for verification
	sessionID, err := generateSecureToken(16)
	if err != nil {
		sendJSONError(w, "Failed to generate session ID", http.StatusInternalServerError)
		return
	}

	sessionToken, err := generateSecureToken(32)
	if err != nil {
		sendJSONError(w, "Failed to generate session token", http.StatusInternalServerError)
		return
	}

	// Create a new transfer session
	session := &TransferSession{
		SessionID:  sessionID,
		Files:      request.Files,
		SenderID:   request.SenderID,
		ReceiverID: request.ReceiverID,
		Status:     "pending",
		CreatedAt:  time.Now(),
		Progress:   0,
		Token:      sessionToken,
	}

	// Store the session
	h.sessionsMu.Lock()
	h.sessions[sessionID] = session
	h.sessionsMu.Unlock()

	// Send the session to the WebSocket clients
	DefaultWebSocketHub.Broadcast("transfer_initiated", map[string]interface{}{
		"sessionId":  sessionID,
		"senderID":   request.SenderID,
		"receiverID": request.ReceiverID,
		"fileCount":  len(request.Files),
		"totalSize":  totalSize,
		"status":     "pending",
	})

	// Send response
	response := models.APIResponse{
		Success: true,
		Message: "Transfer initiated",
		Data: map[string]interface{}{
			"sessionId": sessionID,
			"token":     sessionToken,
			"status":    "pending",
		},
	}

	sendJSONResponse(w, response, http.StatusOK)
}

// HandleAcceptTransfer handles requests to accept a file transfer
func (h *LANTransferHandler) HandleAcceptTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request
	var request struct {
		SessionID string `json:"sessionId"`
		Token     string `json:"token"` // Security token
		Accept    bool   `json:"accept"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		sendJSONError(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate session ID and token
	if request.SessionID == "" || request.Token == "" {
		sendJSONError(w, "Missing sessionId or token", http.StatusBadRequest)
		return
	}

	// Get the session
	h.sessionsMu.RLock()
	session, exists := h.sessions[request.SessionID]
	h.sessionsMu.RUnlock()

	if !exists {
		sendJSONError(w, "Transfer session not found", http.StatusNotFound)
		return
	}

	// Validate the token
	if session.Token != request.Token {
		log.Printf("Invalid token provided for session %s", request.SessionID)
		sendJSONError(w, "Invalid session token", http.StatusUnauthorized)
		return
	}

	// Validate that the session is in the correct state
	if session.Status != "pending" {
		sendJSONError(w, fmt.Sprintf("Session is already %s", session.Status), http.StatusBadRequest)
		return
	}

	// Update the session status
	h.sessionsMu.Lock()
	if request.Accept {
		session.Status = "accepted"
	} else {
		session.Status = "rejected"
	}
	h.sessionsMu.Unlock()

	// Send the updated session to the WebSocket clients
	DefaultWebSocketHub.SendTaskUpdate(request.SessionID, "transfer_status_changed", map[string]interface{}{
		"sessionId": session.SessionID,
		"status":    session.Status,
	})

	// Send response
	response := models.APIResponse{
		Success: true,
		Message: fmt.Sprintf("Transfer %s", session.Status),
		Data: map[string]interface{}{
			"sessionId": session.SessionID,
			"status":    session.Status,
		},
	}

	sendJSONResponse(w, response, http.StatusOK)

	// Start the transfer if accepted
	if request.Accept {
		go h.startTransfer(session)
	}
}

// HandleTransferStatus handles requests to check the status of a transfer
func (h *LANTransferHandler) HandleTransferStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get session ID from query parameters
	sessionID := r.URL.Query().Get("sessionId")
	token := r.URL.Query().Get("token")

	if sessionID == "" {
		sendJSONError(w, "Missing session ID", http.StatusBadRequest)
		return
	}

	// Get the session
	h.sessionsMu.RLock()
	session, exists := h.sessions[sessionID]
	h.sessionsMu.RUnlock()

	if !exists {
		sendJSONError(w, "Transfer session not found", http.StatusNotFound)
		return
	}

	// If token provided, validate it
	if token != "" && session.Token != token {
		sendJSONError(w, "Invalid session token", http.StatusUnauthorized)
		return
	}

	// Create a sanitized version of the session that doesn't include the token
	sanitizedSession := map[string]interface{}{
		"sessionId":   session.SessionID,
		"status":      session.Status,
		"progress":    session.Progress,
		"createdAt":   session.CreatedAt,
		"completedAt": session.CompletedAt,
		"fileCount":   len(session.Files),
		"error":       session.Error,
	}

	// Send response
	response := models.APIResponse{
		Success: true,
		Data:    sanitizedSession,
	}

	sendJSONResponse(w, response, http.StatusOK)
}

// startTransfer starts a file transfer
func (h *LANTransferHandler) startTransfer(session *TransferSession) {
	// Update session status
	h.sessionsMu.Lock()
	session.Status = "transferring"
	h.sessionsMu.Unlock()

	// Send the updated session to the WebSocket clients
	DefaultWebSocketHub.SendTaskUpdate(session.SessionID, "transfer_status_changed", map[string]interface{}{
		"sessionId": session.SessionID,
		"status":    session.Status,
	})

	// Get receiver information
	receiver := h.discoveryService.GetPeer(session.ReceiverID)
	if receiver == nil {
		h.sessionsMu.Lock()
		session.Status = "failed"
		session.Error = "Receiver not found on the LAN"
		h.sessionsMu.Unlock()

		DefaultWebSocketHub.SendTaskUpdate(session.SessionID, "transfer_status_changed", map[string]interface{}{
			"sessionId": session.SessionID,
			"status":    session.Status,
			"error":     session.Error,
		})
		return
	}

	// Check if receiver is still active
	if time.Since(receiver.LastSeen) > 2*time.Minute {
		h.sessionsMu.Lock()
		session.Status = "failed"
		session.Error = "Receiver is no longer active on the LAN"
		h.sessionsMu.Unlock()

		DefaultWebSocketHub.SendTaskUpdate(session.SessionID, "transfer_status_changed", map[string]interface{}{
			"sessionId": session.SessionID,
			"status":    session.Status,
			"error":     session.Error,
		})
		return
	}

	// Total size for progress calculation
	var totalSize int64
	for _, file := range session.Files {
		totalSize += file.Size
	}

	var transferredSize int64
	var transferError error

	// Transfer each file
	for i, file := range session.Files {
		// Skip if file is already transferred
		if i > 0 {
			// Send file started event
			DefaultWebSocketHub.SendTaskUpdate(session.SessionID, "transfer_file_started", map[string]interface{}{
				"sessionId":  session.SessionID,
				"fileName":   file.Name,
				"fileSize":   file.Size,
				"fileIndex":  i,
				"totalFiles": len(session.Files),
			})
		}

		// In a real implementation, we would handle the actual file transfer here
		// For this example, we're just simulating the transfer with progress updates

		// Simulate transfer with some error handling
		chunkSize := file.Size / 10
		if chunkSize < 1 {
			chunkSize = 1
		}

		for j := int64(0); j < 10 && transferredSize < totalSize; j++ {
			// Check for simulated transfer errors (1% chance)
			if j == 5 && (time.Now().UnixNano()%100) == 0 {
				transferError = fmt.Errorf("simulated network error during file transfer")
				break
			}

			// Update transferred size
			currentChunkSize := chunkSize
			if transferredSize+currentChunkSize > totalSize {
				currentChunkSize = totalSize - transferredSize
			}
			transferredSize += currentChunkSize

			// Update progress
			progress := int((transferredSize * 100) / totalSize)
			h.sessionsMu.Lock()
			session.Progress = progress
			h.sessionsMu.Unlock()

			DefaultWebSocketHub.SendTaskUpdate(session.SessionID, "transfer_progress", map[string]interface{}{
				"sessionId":       session.SessionID,
				"progress":        progress,
				"transferredSize": transferredSize,
				"totalSize":       totalSize,
			})

			// Simulate transfer delay
			time.Sleep(100 * time.Millisecond)
		}

		// Break if error occurred
		if transferError != nil {
			break
		}
	}

	// Update session status
	h.sessionsMu.Lock()
	if transferError != nil {
		session.Status = "failed"
		session.Error = transferError.Error()
	} else {
		session.Status = "completed"
		session.Progress = 100
	}
	session.CompletedAt = time.Now()
	h.sessionsMu.Unlock()

	// Send the updated session to the WebSocket clients
	DefaultWebSocketHub.SendTaskUpdate(session.SessionID, "transfer_completed", map[string]interface{}{
		"sessionId": session.SessionID,
		"status":    session.Status,
		"progress":  session.Progress,
		"error":     session.Error,
	})
}

// NewDiscoveryService creates a new discovery service
func NewDiscoveryService() (*DiscoveryService, error) {
	// Setup broadcast address (255.255.255.255:34567)
	broadcastAddr, err := net.ResolveUDPAddr("udp4", "255.255.255.255:34567")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve broadcast address: %w", err)
	}

	// Setup listen address (0.0.0.0:34567)
	listenAddr, err := net.ResolveUDPAddr("udp4", "0.0.0.0:34567")
	if err != nil {
		return nil, fmt.Errorf("failed to resolve listen address: %w", err)
	}

	return &DiscoveryService{
		peers:         make(map[string]*PeerInfo),
		broadcastAddr: broadcastAddr,
		listenAddr:    listenAddr,
		quit:          make(chan struct{}),
		running:       false,
	}, nil
}

// Start starts the discovery service
func (s *DiscoveryService) Start() error {
	// Create a UDP connection for broadcasting
	conn, err := net.ListenUDP("udp4", s.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to create UDP connection: %w", err)
	}
	s.conn = conn

	// Mark as running
	s.running = true

	// Start listening for discovery broadcasts
	go s.listenForDiscovery()

	// Start broadcasting presence
	go s.broadcastPresence()

	// Start cleaning up stale peers
	go s.cleanupStalePeers()

	log.Println("Discovery service started")
	return nil
}

// cleanupStalePeers periodically removes peers that haven't been seen recently
func (s *DiscoveryService) cleanupStalePeers() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.quit:
			return
		case <-ticker.C:
			s.peersMu.Lock()
			now := time.Now()
			for id, peer := range s.peers {
				// Remove peers not seen in the last 10 minutes
				if now.Sub(peer.LastSeen) > 10*time.Minute {
					delete(s.peers, id)
				}
			}
			s.peersMu.Unlock()
		}
	}
}

// Stop stops the discovery service
func (s *DiscoveryService) Stop() {
	// Signal all goroutines to stop
	close(s.quit)

	// Mark as not running
	s.running = false

	// Close the connection
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}

	log.Println("Discovery service stopped")
}

// GetPeers returns all discovered peers
func (s *DiscoveryService) GetPeers() []*PeerInfo {
	s.peersMu.RLock()
	defer s.peersMu.RUnlock()

	peers := make([]*PeerInfo, 0, len(s.peers))
	for _, peer := range s.peers {
		peers = append(peers, peer)
	}

	return peers
}

// GetPeer returns a peer by ID
func (s *DiscoveryService) GetPeer(peerID string) *PeerInfo {
	s.peersMu.RLock()
	defer s.peersMu.RUnlock()

	return s.peers[peerID]
}

// listenForDiscovery listens for discovery broadcasts from peers
func (s *DiscoveryService) listenForDiscovery() {
	buffer := make([]byte, 1024)

	for {
		select {
		case <-s.quit:
			return
		default:
			// Set read deadline to avoid blocking indefinitely
			if s.conn != nil {
				s.conn.SetReadDeadline(time.Now().Add(1 * time.Second))

				// Read from the connection
				n, addr, err := s.conn.ReadFromUDP(buffer)
				if err != nil {
					// Check if it's a timeout error
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						// Just a timeout, continue
						continue
					}

					log.Printf("Error reading UDP: %v", err)
					continue
				}

				// Validate data size to prevent buffer overflow attacks
				if n > 1000 {
					log.Printf("Received oversized packet (%d bytes), ignoring", n)
					continue
				}

				// Parse the peer info with proper error handling
				var peer PeerInfo
				if err := json.Unmarshal(buffer[:n], &peer); err != nil {
					log.Printf("Error parsing peer info: %v", err)
					continue
				}

				// Validate peer info
				if peer.PeerID == "" || peer.Name == "" || peer.Port <= 0 || peer.Port > 65535 {
					log.Printf("Received invalid peer info, ignoring")
					continue
				}

				// Update peer IP from the UDP address
				peer.IP = addr.IP.String()
				peer.LastSeen = time.Now()

				// Store the peer
				s.peersMu.Lock()
				s.peers[peer.PeerID] = &peer
				s.peersMu.Unlock()
			}
		}
	}
}

// broadcastPresence broadcasts presence to peers
func (s *DiscoveryService) broadcastPresence() {
	// Generate a peer ID
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	// Sanitize hostname to prevent injection attacks
	hostname = sanitizeHostname(hostname)

	peerID := fmt.Sprintf("%s-%d", hostname, time.Now().UnixNano())

	// Create peer info
	myInfo := PeerInfo{
		PeerID:     peerID,
		Name:       hostname,
		Port:       34568, // Port for direct file transfer
		DeviceType: "server",
	}

	// Marshal to JSON
	data, err := json.Marshal(myInfo)
	if err != nil {
		log.Printf("Error marshaling peer info: %v", err)
		return
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.quit:
			return
		case <-ticker.C:
			// Send broadcast if connection is available
			if s.conn != nil {
				_, err := s.conn.WriteToUDP(data, s.broadcastAddr)
				if err != nil {
					log.Printf("Error broadcasting presence: %v", err)
				}
			}
		}
	}
}

// sanitizeHostname removes any potentially problematic characters from hostname
func sanitizeHostname(hostname string) string {
	// Remove any non-alphanumeric characters except dashes
	sanitized := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, hostname)

	// Limit length to prevent oversized packets
	if len(sanitized) > 50 {
		return sanitized[:50]
	}
	return sanitized
}

// getClientIP gets the client's IP address from the request
func getClientIP(r *http.Request) string {
	// Check for X-Forwarded-For header
	forwardedFor := r.Header.Get("X-Forwarded-For")
	if forwardedFor != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(forwardedFor, ",")
		return strings.TrimSpace(ips[0])
	}

	// Get IP from RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// Simple in-memory rate limiter
var rateLimiters = struct {
	sync.RWMutex
	limits map[string]map[string]struct {
		count     int
		lastReset time.Time
	}
}{limits: make(map[string]map[string]struct {
	count     int
	lastReset time.Time
})}

// isRateLimited checks if an IP is rate limited for a specific action
func isRateLimited(ip string, action string, limit int) bool {
	rateLimiters.Lock()
	defer rateLimiters.Unlock()

	// Get or create the IP's rate limiter map
	ipLimits, exists := rateLimiters.limits[ip]
	if !exists {
		ipLimits = make(map[string]struct {
			count     int
			lastReset time.Time
		})
		rateLimiters.limits[ip] = ipLimits
	}

	// Get or create the action's rate limiter
	actionLimit, exists := ipLimits[action]
	if !exists || time.Since(actionLimit.lastReset) > time.Minute {
		// Reset if it's been more than a minute
		ipLimits[action] = struct {
			count     int
			lastReset time.Time
		}{1, time.Now()}
		return false
	}

	// Check if limit is exceeded
	if actionLimit.count >= limit {
		return true
	}

	// Increment count
	actionLimit.count++
	ipLimits[action] = actionLimit
	return false
}
