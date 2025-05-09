package handlers

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/example/fileprocessor/internal/models"
)

// LANTransferHandler manages file transfers over the local network
type LANTransferHandler struct {
	sessions    map[string]*TransferSession
	sessionsMu  sync.RWMutex
	discovery   *DiscoveryService
	transferDir string
}

// TransferSession represents an active file transfer session
type TransferSession struct {
	SessionID    string                 `json:"sessionId"`
	Files        []*models.File         `json:"files"`
	Status       string                 `json:"status"`
	Progress     int                    `json:"progress"`
	Error        string                 `json:"error,omitempty"`
	StartedAt    time.Time              `json:"startedAt"`
	CompletedAt  time.Time              `json:"completedAt,omitempty"`
	Peers        []string               `json:"peers,omitempty"`
	SelectedPeer string                 `json:"selectedPeer,omitempty"`
	UserAgent    string                 `json:"userAgent,omitempty"`
	NetworkType  string                 `json:"networkType,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// DiscoveryService handles device discovery on the local network
type DiscoveryService struct {
	peers         map[string]*PeerInfo
	peersMu       sync.RWMutex
	conn          *net.UDPConn
	broadcastAddr *net.UDPAddr
	listenAddr    *net.UDPAddr
	quit          chan struct{}
	running       bool
}

// PeerInfo represents information about a discovered peer
type PeerInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	LastSeen  time.Time `json:"lastSeen"`
	UserAgent string    `json:"userAgent"`
	Type      string    `json:"type"`
}

// PeerMessage is the structure of the broadcast UDP messages
type PeerMessage struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	UserAgent string `json:"userAgent"`
}

// NewLANTransferHandler creates a new LAN transfer handler
func NewLANTransferHandler() (*LANTransferHandler, error) {
	// Create a temporary directory for transfers
	transferDir, err := os.MkdirTemp("", "lan_transfers")
	if err != nil {
		return nil, fmt.Errorf("failed to create transfer directory: %w", err)
	}

	// Create discovery service
	discovery, err := NewDiscoveryService()
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery service: %w", err)
	}

	handler := &LANTransferHandler{
		sessions:    make(map[string]*TransferSession),
		discovery:   discovery,
		transferDir: transferDir,
	}

	// Start discovery service
	go discovery.Start()

	return handler, nil
}

// HandleDiscoverDevices handles requests to discover devices on the network
func (h *LANTransferHandler) HandleDiscoverDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the list of peers
	peers := h.discovery.GetPeers()

	// Convert to JSON response
	response := models.APIResponse{
		Success: true,
		Data:    peers,
	}

	sendJSONResponse(w, response, http.StatusOK)
}

// HandleStartTransfer handles requests to start a file transfer
func (h *LANTransferHandler) HandleStartTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var req struct {
		Files        []*models.File `json:"files"`
		SelectedPeer string         `json:"selectedPeer"`
		UserAgent    string         `json:"userAgent"`
		NetworkType  string         `json:"networkType"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if len(req.Files) == 0 {
		sendJSONError(w, "No files specified", http.StatusBadRequest)
		return
	}

	if req.SelectedPeer == "" {
		sendJSONError(w, "No peer selected", http.StatusBadRequest)
		return
	}

	// Check if the peer exists
	peer := h.discovery.GetPeer(req.SelectedPeer)
	if peer == nil {
		sendJSONError(w, "Selected peer not found", http.StatusBadRequest)
		return
	}

	// Create a session ID
	sessionID := generateSessionID()

	// Create a session
	session := &TransferSession{
		SessionID:    sessionID,
		Files:        req.Files,
		Status:       "pending",
		Progress:     0,
		StartedAt:    time.Now(),
		SelectedPeer: req.SelectedPeer,
		UserAgent:    req.UserAgent,
		NetworkType:  req.NetworkType,
		Metadata:     make(map[string]interface{}),
	}

	// Add the session to the list
	h.sessionsMu.Lock()
	h.sessions[sessionID] = session
	h.sessionsMu.Unlock()

	// Start the transfer in the background
	go h.startTransfer(session, peer)

	// Respond with the session ID
	response := models.APIResponse{
		Success: true,
		Data: map[string]string{
			"sessionId": sessionID,
		},
	}

	sendJSONResponse(w, response, http.StatusOK)
}

// HandleGetTransferStatus handles requests to get the status of a transfer
func (h *LANTransferHandler) HandleGetTransferStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the session ID from the query
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		sendJSONError(w, "Session ID is required", http.StatusBadRequest)
		return
	}

	// Get the session
	h.sessionsMu.RLock()
	session, ok := h.sessions[sessionID]
	h.sessionsMu.RUnlock()

	if !ok {
		sendJSONError(w, "Session not found", http.StatusNotFound)
		return
	}

	// Respond with the session status
	response := models.APIResponse{
		Success: true,
		Data:    session,
	}

	sendJSONResponse(w, response, http.StatusOK)
}

// HandleCancelTransfer handles requests to cancel a transfer
func (h *LANTransferHandler) HandleCancelTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body
	var req struct {
		SessionID string `json:"sessionId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get the session
	h.sessionsMu.Lock()
	session, ok := h.sessions[req.SessionID]
	if !ok {
		h.sessionsMu.Unlock()
		sendJSONError(w, "Session not found", http.StatusNotFound)
		return
	}

	// Update the session status
	session.Status = "canceled"
	session.CompletedAt = time.Now()
	h.sessionsMu.Unlock()

	// Respond with success
	response := models.APIResponse{
		Success: true,
		Data:    session,
	}

	sendJSONResponse(w, response, http.StatusOK)
}

// Stop stops the LAN transfer handler
func (h *LANTransferHandler) Stop() {
	if h.discovery != nil {
		h.discovery.Stop()
	}
}

// Start starts the LAN transfer handler (already started in constructor)
func (h *LANTransferHandler) Start() error {
	// Discovery service is already started in the constructor
	return nil
}

// Helper functions

// generateSessionID generates a unique session ID
func generateSessionID() string {
	// Generate 16 random bytes
	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)

	// Convert to hex string
	return hex.EncodeToString(randomBytes)
}

// startTransfer starts a file transfer session
func (h *LANTransferHandler) startTransfer(session *TransferSession, receiver *PeerInfo) {
	// Update session status
	h.sessionsMu.Lock()
	session.Status = "transferring"
	h.sessionsMu.Unlock()

	// Send initial WebSocket notification
	DefaultWebSocketHub.SendTaskUpdate(session.SessionID, "transfer_started", map[string]interface{}{
		"sessionId": session.SessionID,
		"files":     session.Files,
		"peer":      receiver,
	})

	// Track transfer statistics
	totalSize := int64(0)
	for _, file := range session.Files {
		totalSize += file.Size
	}

	transferredSize := int64(0)
	transferError := error(nil)
	// Get device type for optimized transfer
	deviceType := "other"
	if receiver != nil && receiver.UserAgent != "" {
		deviceType = getDeviceType(receiver.UserAgent)
	}
	networkType := session.NetworkType
	if networkType == "" {
		networkType = "wifi" // Default to WiFi
	}

	// Get optimized chunk size based on device type
	chunkSize := 64 * 1024 // Default to 64KB chunks
	if strings.Contains(strings.ToLower(deviceType), "android") {
		chunkSize = 256 * 1024 // Use 256KB chunks for Android
	}
	// Transfer each file
	for _, file := range session.Files {
		// Calculate number of chunks for this file
		chunkSizeInt64 := int64(chunkSize)
		numChunks := (file.Size + chunkSizeInt64 - 1) / chunkSizeInt64

		// Log the transfer details
		log.Printf("LAN Transfer: optimizing file %s (size: %d) with %d chunks of %d bytes each. Device: %s, Network: %s",
			file.Name, file.Size, numChunks, chunkSize, deviceType, networkType)

		// Transfer in chunks
		for j := int64(0); j < numChunks && transferredSize < totalSize; j++ {
			// Check for simulated transfer errors (0.5% chance, reduced failure rate)
			if j == numChunks/2 && (time.Now().UnixNano()%200) == 0 {
				transferError = fmt.Errorf("simulated network error during file transfer")
				break
			}

			// Update transferred size
			currentChunkSizeInt64 := chunkSizeInt64
			if transferredSize+currentChunkSizeInt64 > totalSize {
				currentChunkSizeInt64 = totalSize - transferredSize
			}
			transferredSize += currentChunkSizeInt64

			// Update progress
			progress := int((transferredSize * 100) / totalSize)
			h.sessionsMu.Lock()
			session.Progress = progress
			h.sessionsMu.Unlock() // Get speed in a way that's compatible with our updated functions
			var speed int64
			if strings.Contains(strings.ToLower(deviceType), "android") {
				// Use legacy function for backward compatibility
				speed = getTransferSpeedLegacy(currentChunkSizeInt64, true)
			} else {
				speed = getTransferSpeedLegacy(currentChunkSizeInt64, false)
			}

			// Format speed for display
			speedFormatted := formatTransferSpeed(float64(speed))

			DefaultWebSocketHub.SendTaskUpdate(session.SessionID, "transfer_progress", map[string]interface{}{
				"sessionId":       session.SessionID,
				"progress":        progress,
				"transferredSize": transferredSize,
				"totalSize":       totalSize,
				"speed":           speed,
				"speedFormatted":  speedFormatted,
				"deviceType":      deviceType,
			})

			// Simulate transfer delay - faster for Android devices
			if strings.Contains(strings.ToLower(deviceType), "android") {
				time.Sleep(50 * time.Millisecond) // Faster updates for Android
			} else {
				time.Sleep(100 * time.Millisecond)
			}
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

	// Start listening for broadcasts
	go s.listen()

	// Start broadcasting presence
	go s.broadcast()

	return nil
}

// Stop stops the discovery service
func (s *DiscoveryService) Stop() {
	if !s.running {
		return
	}

	// Signal quit
	close(s.quit)

	// Close the connection
	if s.conn != nil {
		s.conn.Close()
	}

	// Mark as not running
	s.running = false
}

// GetPeers returns a list of discovered peers
func (s *DiscoveryService) GetPeers() []*PeerInfo {
	s.peersMu.RLock()
	defer s.peersMu.RUnlock()

	// Convert map to slice
	peers := make([]*PeerInfo, 0, len(s.peers))
	for _, peer := range s.peers {
		peers = append(peers, peer)
	}

	return peers
}

// GetPeer returns a specific peer by ID
func (s *DiscoveryService) GetPeer(id string) *PeerInfo {
	s.peersMu.RLock()
	defer s.peersMu.RUnlock()

	return s.peers[id]
}

// listen listens for broadcast messages from other peers
func (s *DiscoveryService) listen() {
	buffer := make([]byte, 1024)

	for {
		select {
		case <-s.quit:
			return
		default:
			// Set a read deadline to avoid blocking forever
			s.conn.SetReadDeadline(time.Now().Add(1 * time.Second))

			// Read from the connection
			n, addr, err := s.conn.ReadFromUDP(buffer)
			if err != nil {
				// Check if it's a timeout
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				log.Printf("Error reading from UDP: %v", err)
				continue
			}

			// Parse the message
			var message PeerMessage
			if err := json.Unmarshal(buffer[:n], &message); err != nil {
				log.Printf("Error parsing peer message: %v", err)
				continue
			}

			// Update peer list
			s.updatePeer(message, addr.String())
		}
	}
}

// broadcast broadcasts presence to other peers
func (s *DiscoveryService) broadcast() {
	// Create our peer message
	message := PeerMessage{
		Type:      "presence",
		ID:        generatePeerID(),
		Name:      getHostname(),
		UserAgent: "FileProcessor/1.0",
	}

	// Convert to JSON
	messageData, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error encoding peer message: %v", err)
		return
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.quit:
			return
		case <-ticker.C:
			// Send broadcast
			_, err := s.conn.WriteToUDP(messageData, s.broadcastAddr)
			if err != nil {
				log.Printf("Error sending broadcast: %v", err)
			}
		}
	}
}

// updatePeer updates the peer list with a new peer
func (s *DiscoveryService) updatePeer(message PeerMessage, address string) {
	// Don't add ourselves
	if message.ID == generatePeerID() {
		return
	}

	s.peersMu.Lock()
	defer s.peersMu.Unlock()

	// Check if peer already exists
	peer, exists := s.peers[message.ID]
	if exists {
		// Update existing peer
		peer.LastSeen = time.Now()
		peer.Name = message.Name
		peer.UserAgent = message.UserAgent
	} else {
		// Add new peer
		s.peers[message.ID] = &PeerInfo{
			ID:        message.ID,
			Name:      message.Name,
			Address:   address,
			LastSeen:  time.Now(),
			UserAgent: message.UserAgent,
			Type:      getDeviceType(message.UserAgent),
		}
	}
}

// generatePeerID generates a unique ID for this peer
func generatePeerID() string {
	// We'll use the hostname and MAC address to create a unique ID
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	return fmt.Sprintf("%s-%d", hostname, time.Now().UnixNano())
}

// getHostname gets the hostname of this device
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "Unknown Device"
	}
	return hostname
}

// getDeviceType determines the device type from the user agent
func getDeviceType(userAgent string) string {
	userAgent = strings.ToLower(userAgent)
	if strings.Contains(userAgent, "android") {
		return "android"
	} else if strings.Contains(userAgent, "ios") || strings.Contains(userAgent, "iphone") || strings.Contains(userAgent, "ipad") {
		return "ios"
	} else if strings.Contains(userAgent, "windows") {
		return "windows"
	} else if strings.Contains(userAgent, "mac") {
		return "mac"
	} else {
		return "other"
	}
}

// This function has been moved to transfer_helpers.go
