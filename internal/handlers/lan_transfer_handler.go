package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
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

	return &LANTransferHandler{
		sessions:         make(map[string]*TransferSession),
		discoveryService: discoveryService,
	}, nil
}

// Start starts the LAN transfer handler
func (h *LANTransferHandler) Start() error {
	// Start the discovery service
	if err := h.discoveryService.Start(); err != nil {
		return fmt.Errorf("failed to start discovery service: %w", err)
	}

	log.Println("LAN transfer service started")
	return nil
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

	// Get all currently discovered peers
	peers := h.discoveryService.GetPeers()

	// Send response
	response := models.APIResponse{
		Success: true,
		Data:    peers,
	}

	sendJSONResponse(w, response, http.StatusOK)
}

// HandleInitiateTransfer handles requests to initiate a file transfer
func (h *LANTransferHandler) HandleInitiateTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

	// Check if the receiver exists
	receiverInfo := h.discoveryService.GetPeer(request.ReceiverID)
	if receiverInfo == nil {
		sendJSONError(w, "Receiver not found on the LAN", http.StatusNotFound)
		return
	}

	// Create a new transfer session
	sessionID := fmt.Sprintf("transfer-%d", time.Now().UnixNano())
	session := &TransferSession{
		SessionID:  sessionID,
		Files:      request.Files,
		SenderID:   request.SenderID,
		ReceiverID: request.ReceiverID,
		Status:     "pending",
		CreatedAt:  time.Now(),
		Progress:   0,
	}

	// Store the session
	h.sessionsMu.Lock()
	h.sessions[sessionID] = session
	h.sessionsMu.Unlock()

	// Send the session to the WebSocket clients
	DefaultWebSocketHub.Broadcast("transfer_initiated", session)

	// Send response
	response := models.APIResponse{
		Success: true,
		Message: "Transfer initiated",
		Data:    session,
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
		Accept    bool   `json:"accept"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		sendJSONError(w, "Invalid request format", http.StatusBadRequest)
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

	// Update the session status
	h.sessionsMu.Lock()
	if request.Accept {
		session.Status = "accepted"
	} else {
		session.Status = "rejected"
	}
	h.sessionsMu.Unlock()

	// Send the updated session to the WebSocket clients
	DefaultWebSocketHub.SendTaskUpdate(request.SessionID, "transfer_status_changed", session)

	// Send response
	response := models.APIResponse{
		Success: true,
		Message: fmt.Sprintf("Transfer %s", session.Status),
		Data:    session,
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

	// Send response
	response := models.APIResponse{
		Success: true,
		Data:    session,
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
	DefaultWebSocketHub.SendTaskUpdate(session.SessionID, "transfer_status_changed", session)

	// Get receiver information
	receiver := h.discoveryService.GetPeer(session.ReceiverID)
	if receiver == nil {
		h.sessionsMu.Lock()
		session.Status = "failed"
		session.Error = "Receiver not found on the LAN"
		h.sessionsMu.Unlock()

		DefaultWebSocketHub.SendTaskUpdate(session.SessionID, "transfer_status_changed", session)
		return
	}

	// Total size for progress calculation
	var totalSize int64
	for _, file := range session.Files {
		totalSize += file.Size
	}

	var transferredSize int64

	// Transfer each file
	for i, file := range session.Files {
		// Skip if file is already transferred
		if i > 0 {
			DefaultWebSocketHub.SendTaskUpdate(session.SessionID, "transfer_file_started", map[string]interface{}{
				"fileName":   file.Name,
				"fileSize":   file.Size,
				"fileIndex":  i,
				"totalFiles": len(session.Files),
			})
		}

		// Update progress
		progress := int((transferredSize * 100) / totalSize)
		h.sessionsMu.Lock()
		session.Progress = progress
		h.sessionsMu.Unlock()

		DefaultWebSocketHub.SendTaskUpdate(session.SessionID, "transfer_progress", map[string]interface{}{
			"progress":        progress,
			"transferredSize": transferredSize,
			"totalSize":       totalSize,
		})

		time.Sleep(100 * time.Millisecond) // Simulate transfer time

		transferredSize += file.Size
	}

	// Update session status
	h.sessionsMu.Lock()
	session.Status = "completed"
	session.Progress = 100
	session.CompletedAt = time.Now()
	h.sessionsMu.Unlock()

	// Send the updated session to the WebSocket clients
	DefaultWebSocketHub.SendTaskUpdate(session.SessionID, "transfer_completed", session)
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

	// Start listening for discovery broadcasts
	go s.listenForDiscovery()

	// Start broadcasting presence
	go s.broadcastPresence()

	log.Println("Discovery service started")
	return nil
}

// Stop stops the discovery service
func (s *DiscoveryService) Stop() {
	if s.conn != nil {
		s.conn.Close()
	}

	// Signal all goroutines to stop
	close(s.quit)

	log.Println("Discovery service stopped")
}

// GetPeers returns all discovered peers
func (s *DiscoveryService) GetPeers() []*PeerInfo {
	s.peersMu.RLock()
	defer s.peersMu.RUnlock()

	// Filter out stale peers (older than 1 minute)
	cutoffTime := time.Now().Add(-1 * time.Minute)

	var peers []*PeerInfo
	for _, peer := range s.peers {
		if peer.LastSeen.After(cutoffTime) {
			peers = append(peers, peer)
		}
	}

	return peers
}

// GetPeer returns a peer by ID
func (s *DiscoveryService) GetPeer(peerID string) *PeerInfo {
	s.peersMu.RLock()
	defer s.peersMu.RUnlock()

	peer, exists := s.peers[peerID]
	if !exists {
		return nil
	}

	// Check if the peer is stale (older than 1 minute)
	if time.Now().Sub(peer.LastSeen) > 1*time.Minute {
		return nil
	}

	return peer
}

// listenForDiscovery listens for discovery broadcasts from peers
func (s *DiscoveryService) listenForDiscovery() {
	buffer := make([]byte, 1024)

	for {
		select {
		case <-s.quit:
			return
		default:
			// Read from the connection
			n, addr, err := s.conn.ReadFromUDP(buffer)
			if err != nil {
				log.Printf("Error reading UDP: %v", err)
				continue
			}

			// Parse the peer info
			var peer PeerInfo
			if err := json.Unmarshal(buffer[:n], &peer); err != nil {
				log.Printf("Error parsing peer info: %v", err)
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

// broadcastPresence broadcasts presence to peers
func (s *DiscoveryService) broadcastPresence() {
	// Generate a peer ID
	hostname, _ := os.Hostname()
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
			// Send broadcast
			_, err := s.conn.WriteToUDP(data, s.broadcastAddr)
			if err != nil {
				log.Printf("Error broadcasting presence: %v", err)
			}
		}
	}
}
