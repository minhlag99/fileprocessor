package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections (you may want to restrict this in production)
	},
}

// ClientMessage represents a message from a client
type ClientMessage struct {
	Type    string          `json:"type"`
	TaskID  string          `json:"taskId,omitempty"`
	Content json.RawMessage `json:"content,omitempty"`
}

// ServerMessage represents a message to a client
type ServerMessage struct {
	Type    string      `json:"type"`
	TaskID  string      `json:"taskId,omitempty"`
	Content interface{} `json:"content,omitempty"`
}

// Client represents a connected websocket client
type Client struct {
	conn      *websocket.Conn
	send      chan ServerMessage
	clientID  string
	tasks     []string
	hub       *WebSocketHub
	lastPing  time.Time
	closeMu   sync.Mutex
	isClosed  bool
}

// WebSocketHub manages all connected clients
type WebSocketHub struct {
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan ServerMessage
	tasks      map[string][]string // taskID -> clientIDs
	mu         sync.RWMutex
}

// NewWebSocketHub creates a new websocket hub
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan ServerMessage),
		tasks:      make(map[string][]string),
	}
}

// Run runs the websocket hub in a goroutine
func (h *WebSocketHub) Run() {
	go func() {
		for {
			select {
			case client := <-h.register:
				h.mu.Lock()
				h.clients[client.clientID] = client
				h.mu.Unlock()
				
				// Send welcome message
				client.send <- ServerMessage{
					Type:    "connected",
					Content: map[string]string{"message": "Connected to file processing server"},
				}

			case client := <-h.unregister:
				h.mu.Lock()
				if _, ok := h.clients[client.clientID]; ok {
					delete(h.clients, client.clientID)
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
	
	// Start a goroutine to check for inactive connections
	go h.checkInactiveConnections()
}

// Subscribe subscribes a client to updates for a specific task
func (h *WebSocketHub) Subscribe(clientID, taskID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// Add client to task subscribers
	if _, exists := h.tasks[taskID]; !exists {
		h.tasks[taskID] = make([]string, 0)
	}
	
	// Check if already subscribed
	for _, id := range h.tasks[taskID] {
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
	h.broadcast <- ServerMessage{
		Type:    updateType,
		TaskID:  taskID,
		Content: content,
	}
}

// Broadcast sends a message to all connected clients
func (h *WebSocketHub) Broadcast(messageType string, content interface{}) {
	h.broadcast <- ServerMessage{
		Type:    messageType,
		Content: content,
	}
}

// checkInactiveConnections periodically checks for inactive connections
func (h *WebSocketHub) checkInactiveConnections() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		<-ticker.C
		
		now := time.Now()
		timeout := 2 * time.Minute
		
		h.mu.RLock()
		for _, client := range h.clients {
			if now.Sub(client.lastPing) > timeout {
				go func(c *Client) {
					h.unregister <- c
				}(client)
			}
		}
		h.mu.RUnlock()
	}
}

// ServeWs handles websocket requests from clients
func ServeWs(hub *WebSocketHub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading connection: %v", err)
		return
	}
	
	// Generate a unique client ID
	clientID := r.RemoteAddr + "-" + time.Now().Format(time.RFC3339Nano)
	
	client := &Client{
		conn:     conn,
		send:     make(chan ServerMessage, 256),
		clientID: clientID,
		tasks:    make([]string, 0),
		hub:      hub,
		lastPing: time.Now(),
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
	
	c.conn.SetReadLimit(512 * 1024) // 512KB max message size
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		c.lastPing = time.Now()
		return nil
	})
	
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Error reading message: %v", err)
			}
			break
		}
		
		// Parse client message
		var clientMsg ClientMessage
		if err := json.Unmarshal(message, &clientMsg); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}
		
		// Handle message based on type
		switch clientMsg.Type {
		case "subscribe":
			if clientMsg.TaskID != "" {
				c.hub.Subscribe(c.clientID, clientMsg.TaskID)
				c.send <- ServerMessage{
					Type:   "subscribed",
					TaskID: clientMsg.TaskID,
				}
			}
		
		case "unsubscribe":
			if clientMsg.TaskID != "" {
				c.hub.Unsubscribe(c.clientID, clientMsg.TaskID)
				c.send <- ServerMessage{
					Type:   "unsubscribed",
					TaskID: clientMsg.TaskID,
				}
			}
			
		case "ping":
			c.lastPing = time.Now()
			c.send <- ServerMessage{
				Type: "pong",
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
				// Channel closed
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
			
			w.Write(jsonData)
			
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