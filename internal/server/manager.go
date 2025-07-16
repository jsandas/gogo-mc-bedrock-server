package server

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WrapperStatus represents the current status of a wrapper connection
type WrapperStatus string

const (
	StatusDisconnected WrapperStatus = "disconnected"
	StatusConnecting   WrapperStatus = "connecting"
	StatusConnected    WrapperStatus = "connected"
	StatusError        WrapperStatus = "error"
	StatusReconnecting WrapperStatus = "reconnecting"

	maxReconnectAttempts = 5
	reconnectDelay       = 5 * time.Second
)

// ConnectionStats tracks connection statistics
type ConnectionStats struct {
	ConnectedAt      time.Time `json:"connected_at,omitempty"`
	LastMessageAt    time.Time `json:"last_message_at,omitempty"`
	MessagesSent     int64     `json:"messages_sent"`
	MessagesReceived int64     `json:"messages_received"`
	Reconnections    int       `json:"reconnections"`
}

// WrapperConnection represents a connection to a remote Minecraft server wrapper
type WrapperConnection struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Address  string          `json:"address"`
	Username string          `json:"-"` // Hide sensitive info from JSON
	Password string          `json:"-"`
	Status   WrapperStatus   `json:"status"`
	Error    string          `json:"error,omitempty"`
	Stats    ConnectionStats `json:"stats"`

	conn            *websocket.Conn
	sendChan        chan []byte
	recvChan        chan []byte
	clients         map[*websocket.Conn]bool
	clientsMu       sync.RWMutex
	done            chan struct{}
	reconnectSignal chan struct{}
	reconnectMu     sync.Mutex
	statsMu         sync.RWMutex
}

// ConnectionManager manages multiple wrapper connections
type ConnectionManager struct {
	connections map[string]*WrapperConnection
	mu          sync.RWMutex
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]*WrapperConnection),
	}
}

// Connect establishes a connection to a remote wrapper
func (m *ConnectionManager) Connect(id, name, address, username, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if connection already exists
	if _, exists := m.connections[id]; exists {
		return fmt.Errorf("connection with ID %s already exists", id)
	}

	// Create new connection
	wConn := &WrapperConnection{
		ID:              id,
		Name:            name,
		Address:         address,
		Username:        username,
		Password:        password,
		Status:          StatusConnecting,
		sendChan:        make(chan []byte, 100),
		recvChan:        make(chan []byte, 100),
		clients:         make(map[*websocket.Conn]bool),
		done:            make(chan struct{}),
		reconnectSignal: make(chan struct{}),
	}

	m.connections[id] = wConn

	// Start connection management goroutine
	go wConn.manage()

	return nil
}

// GetConnection returns a connection by ID
func (m *ConnectionManager) GetConnection(id string) (*WrapperConnection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	conn, exists := m.connections[id]
	return conn, exists
}

// ListConnections returns a list of all active connections
func (m *ConnectionManager) ListConnections() []*WrapperConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	conns := make([]*WrapperConnection, 0, len(m.connections))
	for _, conn := range m.connections {
		conns = append(conns, conn)
	}
	return conns
}

// manage handles the connection lifecycle including automatic reconnection
func (w *WrapperConnection) manage() {
	var reconnectAttempts int

	for {
		err := w.connect()
		if err == nil {
			// Reset reconnect attempts on successful connection
			reconnectAttempts = 0
			// Wait for connection to fail
			<-w.reconnectSignal
		}

		select {
		case <-w.done:
			return
		default:
			if reconnectAttempts >= maxReconnectAttempts {
				w.Status = StatusError
				w.Error = "max reconnection attempts reached"
				return
			}

			reconnectAttempts++
			w.Status = StatusReconnecting
			time.Sleep(reconnectDelay * time.Duration(reconnectAttempts))
		}
	}
}

// connect establishes a connection to the wrapper
func (w *WrapperConnection) connect() error {
	w.reconnectMu.Lock()
	defer w.reconnectMu.Unlock()

	// Create auth header if credentials are provided
	header := http.Header{}
	if w.Username != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(w.Username + ":" + w.Password))
		header.Set("Authorization", "Basic "+auth)
	}

	// Connect to the wrapper
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, resp, err := dialer.Dial(w.Address, header)
	if err != nil {
		w.Status = StatusError
		errMsg := err.Error()
		if resp != nil {
			errMsg = fmt.Sprintf("%v (HTTP Status: %d)", err, resp.StatusCode)
		}
		w.Error = errMsg
		return fmt.Errorf("failed to connect to wrapper: %v", errMsg)
	}

	w.conn = conn
	w.Status = StatusConnected
	w.statsMu.Lock()
	w.Stats.ConnectedAt = time.Now()
	w.Stats.Reconnections++
	w.statsMu.Unlock()

	// Start message handling goroutines
	go w.readPump()
	go w.writePump()

	return nil
}

// readPump pumps messages from the wrapper connection to all connected clients
func (w *WrapperConnection) readPump() {
	defer func() {
		w.conn.Close()
		// Signal for reconnection
		select {
		case w.reconnectSignal <- struct{}{}:
		default:
		}
	}()

	w.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	w.conn.SetPongHandler(func(string) error {
		w.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := w.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("Wrapper connection error: %v\n", err)
			}
			w.Status = StatusError
			w.Error = err.Error()
			break
		}

		// Update stats
		w.statsMu.Lock()
		w.Stats.MessagesReceived++
		w.Stats.LastMessageAt = time.Now()
		w.statsMu.Unlock()

		// Broadcast message to all connected clients
		w.clientsMu.RLock()
		for client := range w.clients {
			if err := client.WriteMessage(websocket.TextMessage, message); err != nil {
				fmt.Printf("Error writing to client: %v\n", err)
				w.RemoveClient(client)
			}
		}
		w.clientsMu.RUnlock()
	}
}

// writePump pumps messages from the clients to the wrapper connection
func (w *WrapperConnection) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		w.conn.Close()
	}()

	for {
		select {
		case message, ok := <-w.sendChan:
			w.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Channel closed
				w.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := w.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				fmt.Printf("Error writing to wrapper: %v\n", err)
				return
			}

			// Update stats
			w.statsMu.Lock()
			w.Stats.MessagesSent++
			w.Stats.LastMessageAt = time.Now()
			w.statsMu.Unlock()

		case <-ticker.C:
			w.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := w.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-w.done:
			return
		}
	}
}

// AddClient adds a web client connection to this wrapper
func (w *WrapperConnection) AddClient(client *websocket.Conn) {
	w.clientsMu.Lock()
	w.clients[client] = true
	w.clientsMu.Unlock()
}

// RemoveClient removes a web client connection
func (w *WrapperConnection) RemoveClient(client *websocket.Conn) {
	w.clientsMu.Lock()
	delete(w.clients, client)
	w.clientsMu.Unlock()
}

// SendMessage sends a message to the wrapper
func (w *WrapperConnection) SendMessage(message []byte) error {
	select {
	case w.sendChan <- message:
		return nil
	default:
		return fmt.Errorf("message buffer full")
	}
}

// DisconnectAll closes all wrapper connections
func (m *ConnectionManager) DisconnectAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, wConn := range m.connections {
		if wConn.conn != nil {
			wConn.conn.Close()
		}
		close(wConn.done)
		delete(m.connections, id)
		fmt.Printf("Disconnected from wrapper %s (%s)\n", wConn.Name, wConn.ID)
	}
}
