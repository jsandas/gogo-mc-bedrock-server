package server

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jsandas/gogo-mc-bedrock-server/internal/raknet"
)

// WrapperStatus represents the current status of a wrapper connection.
type WrapperStatus string

const (
	StatusDisconnected WrapperStatus = "disconnected"
	StatusConnecting   WrapperStatus = "connecting"
	StatusConnected    WrapperStatus = "connected"
	StatusError        WrapperStatus = "error"
	StatusReconnecting WrapperStatus = "reconnecting"

	StatusAuthFailed = "authentication failed"

	maxReconnectAttempts = 5
	reconnectDelay       = 5 * time.Second
)

// ConnectionStats tracks connection statistics.
type ConnectionStats struct {
	ConnectedAt      time.Time `json:"connected_at,omitempty"`
	LastMessageAt    time.Time `json:"last_message_at,omitempty"`
	MessagesSent     int64     `json:"messages_sent"`
	MessagesReceived int64     `json:"messages_received"`
	Reconnections    int       `json:"reconnections"`
}

// WrapperConnection represents a connection to a remote Minecraft server wrapper.
type WrapperConnection struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Address   string          `json:"address"`
	Username  string          `json:"-"` // Hide sensitive info from JSON
	Password  string          `json:"-"`
	SharedKey string          `json:"-"` // Auth key for the wrapper
	Status    WrapperStatus   `json:"status"`
	Error     string          `json:"error,omitempty"`
	Stats     ConnectionStats `json:"stats"`

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

// ConnectionManager manages multiple wrapper connections.
type ConnectionManager struct {
	connections map[string]*WrapperConnection
	mu          sync.RWMutex
}

// NewConnectionManager creates a new connection manager.
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]*WrapperConnection),
	}
}

// Connect establishes a connection to a remote wrapper.
func (m *ConnectionManager) Connect(id, name, address, username, password, sharedKey string) error {
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
		SharedKey:       sharedKey,
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

// GetConnection returns a connection by ID.
func (m *ConnectionManager) GetConnection(id string) (*WrapperConnection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, exists := m.connections[id]

	return conn, exists
}

// ListConnections returns a list of all active connections.
func (m *ConnectionManager) ListConnections() []*WrapperConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conns := make([]*WrapperConnection, 0, len(m.connections))
	for _, conn := range m.connections {
		conns = append(conns, conn)
	}

	return conns
}

// Retry initiates a manual reconnection attempt.
func (w *WrapperConnection) Retry() error {
	w.reconnectMu.Lock()
	defer w.reconnectMu.Unlock()

	// Don't retry if we're already connected or connecting
	if w.Status == StatusConnected || w.Status == StatusConnecting {
		return fmt.Errorf("connection is already %s", w.Status)
	}

	// Signal the manage routine to retry
	select {
	case w.reconnectSignal <- struct{}{}:
		w.Status = StatusConnecting
		return nil
	case <-w.done:
		return fmt.Errorf("connection is closed")
	default:
		return fmt.Errorf("retry already in progress")
	}
}

// AddClient adds a web client connection to this wrapper.
func (w *WrapperConnection) AddClient(client *websocket.Conn) {
	w.clientsMu.Lock()
	w.clients[client] = true
	w.clientsMu.Unlock()
}

// RemoveClient removes a web client connection.
func (w *WrapperConnection) RemoveClient(client *websocket.Conn) {
	w.clientsMu.Lock()
	delete(w.clients, client)
	w.clientsMu.Unlock()
}

// SendMessage sends a message to the wrapper.
func (w *WrapperConnection) SendMessage(message []byte) error {
	if w.Status != StatusConnected {
		return fmt.Errorf("wrapper is not connected (status: %s)", w.Status)
	}

	select {
	case w.sendChan <- message:
		return nil
	case <-w.done:
		return fmt.Errorf("connection is closed")
	default:
		return fmt.Errorf("message buffer full")
	}
}

// GetServerStatus gets the current Minecraft server status using GetPong.
func (w *WrapperConnection) GetServerStatus() (map[string]interface{}, error) {
	// Extract host from the address
	addr := w.Address
	if addr == "" {
		return nil, fmt.Errorf("wrapper address is empty")
	}

	// Convert from ws:// to regular address and extract host
	addr = strings.TrimPrefix(addr, "ws://")
	addr = strings.TrimSuffix(addr, "/ws")

	// Split host and port
	host := addr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		host = addr[:idx]
	}

	if host == "localhost" {
		host = "127.0.0.1"
	}

	// Check if we have a custom port from environment
	serverPort := "19132" // Default Minecraft Bedrock port
	if port := os.Getenv("CFG_SERVER_PORT"); port != "" {
		serverPort = port
	}

	// Combine host and Minecraft server port
	mcAddr := fmt.Sprintf("%s:%s", host, serverPort)

	pong, err := raknet.GetPong(mcAddr)
	if err != nil {
		return nil, fmt.Errorf("error getting server status from %s: %v", mcAddr, err)
	}

	return map[string]interface{}{
		"serverName":     pong.ServerName,
		"versionName":    pong.VersionName,
		"levelName":      pong.LevelName,
		"gameMode":       pong.GameMode,
		"playerCount":    pong.PlayerCount,
		"maxPlayerCount": pong.MaxPlayerCount,
	}, nil
}

// DisconnectAll closes all wrapper connections.
func (m *ConnectionManager) DisconnectAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, wConn := range m.connections {
		if wConn.conn != nil {
			err := wConn.conn.Close()
			if err != nil {
				fmt.Printf("Error closing connection: %v\n", err)
			}
		}

		close(wConn.done)
		delete(m.connections, id)
		fmt.Printf("Disconnected from wrapper %s (%s)\n", wConn.Name, wConn.ID)
	}
}

// manage handles the connection lifecycle including automatic reconnection.
func (w *WrapperConnection) manage() {
	var reconnectAttempts int

	for {
		err := w.connect()
		if err != nil {
			w.Status = StatusError

			// If authentication failed, don't retry
			if err.Error() == StatusAuthFailed {
				w.Error = StatusAuthFailed
				return
			}

			// Set error message and continue with reconnection
			w.Error = err.Error()

			select {
			case <-w.done:
				return
			case <-w.reconnectSignal:
				// Manual retry requested, reset attempts
				reconnectAttempts = 0
				continue
			default:
				if reconnectAttempts >= maxReconnectAttempts {
					w.Error = "max reconnection attempts reached. Click retry to try again."
					// Wait for manual retry
					select {
					case <-w.done:
						return
					case <-w.reconnectSignal:
						reconnectAttempts = 0
						continue
					}
				}

				reconnectAttempts++
				w.Status = StatusReconnecting

				time.Sleep(reconnectDelay * time.Duration(reconnectAttempts))

				continue
			}
		}

		// Reset reconnect attempts on successful connection
		reconnectAttempts = 0
		w.Error = "" // Clear any previous error
		w.Status = StatusConnected

		// Wait for connection to fail or manual retry
		select {
		case <-w.reconnectSignal:
			// Manual reconnect requested
			if w.conn != nil {
				err := w.conn.Close()
				if err != nil {
					fmt.Printf("Error closing connection: %v\n", err)
				}
			}

			continue
		case <-w.done:
			return
		}
	}
}

// connect establishes a connection to the wrapper.
func (w *WrapperConnection) connect() error {
	w.reconnectMu.Lock()
	defer w.reconnectMu.Unlock()

	// Create auth header if credentials are provided
	header := http.Header{}

	if w.Username != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(w.Username + ":" + w.Password))
		header.Set("Authorization", "Basic "+auth)
	}

	// Add the shared key header
	if w.SharedKey != "" {
		header.Set("X-Auth-Key", w.SharedKey)
	}

	// Connect to the wrapper
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	// Check if there's already an active connection
	if w.conn != nil {
		err := w.conn.Close()
		if err != nil {
			fmt.Printf("Error closing existing connection: %v\n", err)
		}
		w.conn = nil
	}

	conn, resp, err := dialer.Dial(w.Address, header)
	if err != nil {
		w.Status = StatusError
		errMsg := err.Error()

		if resp != nil {
			if resp.StatusCode == http.StatusUnauthorized {
				w.Error = StatusAuthFailed
				return fmt.Errorf("%s", StatusAuthFailed)
			}

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

// readPump pumps messages from the wrapper connection to all connected clients.
func (w *WrapperConnection) readPump() {
	defer func() {
		w.Status = StatusDisconnected
		if w.conn != nil {
			err := w.conn.Close()
			if err != nil {
				fmt.Printf("Error closing connection: %v\n", err)
			}
		}
		// Signal for reconnection
		select {
		case w.reconnectSignal <- struct{}{}:
		default:
		}
	}()

	if w.conn == nil {
		w.Error = "connection is nil"

		return
	}

	err := w.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	if err != nil {
		fmt.Printf("Error setting read deadline: %v\n", err)
		return
	}

	w.conn.SetPongHandler(func(string) error {
		if w.conn != nil {
			err := w.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			if err != nil {
				fmt.Printf("Error setting read deadline: %v\n", err)
				return err
			}
		}

		return nil
	})

	for {
		_, message, err := w.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("Wrapper connection error: %v\n", err)
			}

			w.Status = StatusError
			w.Error = fmt.Sprintf("read error: %v", err)

			return
		}

		// Update stats
		w.statsMu.Lock()
		w.Stats.MessagesReceived++
		w.Stats.LastMessageAt = time.Now()
		w.statsMu.Unlock()

		// Broadcast message to all connected clients
		w.clientsMu.RLock()

		for client := range w.clients {
			err := client.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				fmt.Printf("Error writing to client: %v\n", err)
				err = client.Close()
				if err != nil {
					fmt.Printf("Error closing client connection: %v\n", err)
				}
				w.RemoveClient(client)
			}
		}

		w.clientsMu.RUnlock()
	}
}

// writePump pumps messages from the clients to the wrapper connection.
func (w *WrapperConnection) writePump() {
	ticker := time.NewTicker(54 * time.Second)

	defer func() {
		ticker.Stop()

		if w.conn != nil {
			err := w.conn.WriteMessage(websocket.CloseMessage, []byte{})
			if err != nil {
				fmt.Printf("Error sending close message: %v\n", err)
				return
			}

			err = w.conn.Close()
			if err != nil {
				fmt.Printf("Error closing connection: %v\n", err)
			}
		}

		w.Status = StatusDisconnected
		// Signal reconnection needed
		select {
		case w.reconnectSignal <- struct{}{}:
		default:
		}
	}()

	for {
		select {
		case message, ok := <-w.sendChan:
			if !ok {
				// Channel closed
				return
			}

			if w.conn == nil {
				fmt.Printf("Connection lost while trying to write message\n")

				return
			}

			err := w.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err != nil {
				fmt.Printf("Error setting write deadline: %v\n", err)
				return
			}

			err = w.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				fmt.Printf("Error writing to wrapper: %v\n", err)
				w.Error = fmt.Sprintf("write error: %v", err)

				return
			}

			// Update stats
			w.statsMu.Lock()
			w.Stats.MessagesSent++
			w.Stats.LastMessageAt = time.Now()
			w.statsMu.Unlock()

		case <-ticker.C:
			if w.conn == nil {
				return
			}

			err := w.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err != nil {
				fmt.Printf("Error setting write deadline: %v\n", err)
				return
			}

			err = w.conn.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				fmt.Printf("Ping failed: %v\n", err)
				return
			}

		case <-w.done:
			return
		}
	}
}
