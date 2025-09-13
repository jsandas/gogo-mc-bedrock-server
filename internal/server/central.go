package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// CentralServerConfig holds configuration for the central server.
type CentralServerConfig struct {
	Manager *ConnectionManager
	AuthKey string
}

// CentralServer represents the central management server.
type CentralServer struct {
	manager    *ConnectionManager
	server     *http.Server
	upgrader   websocket.Upgrader
	clients    map[*websocket.Conn]bool
	clientsMux sync.RWMutex
	authKey    string
}

// NewCentralServer creates a new central server instance.
func NewCentralServer(config CentralServerConfig) *CentralServer {
	return &CentralServer{
		manager: config.Manager,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for now
			},
		},
		clients: make(map[*websocket.Conn]bool),
		authKey: config.AuthKey,
	}
}

// Start starts the HTTP server.
func (s *CentralServer) Start(addr string) error {
	mux := http.NewServeMux()

	// Public routes
	mux.Handle("/", http.FileServer(http.Dir("web")))

	// Protected routes
	mux.HandleFunc("/api/wrappers", s.authMiddleware(s.handleWrappers))
	mux.HandleFunc("/api/retry", s.authMiddleware(s.handleRetry))
	mux.HandleFunc("/api/serverstatus", s.authMiddleware(s.handleServerStatus))
	mux.HandleFunc("/ws", s.authMiddleware(s.handleWebSocket))

	s.server = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}

	return s.server.ListenAndServe()
}

// Stop gracefully shuts down the server.
func (s *CentralServer) Stop() error {
	return s.server.Shutdown(context.Background())
}

// handleWrappers handles requests for wrapper information.
func (s *CentralServer) handleWrappers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	wrappers := s.manager.ListConnections()
	json.NewEncoder(w).Encode(wrappers)
}

// handleServerStatus handles requests for Minecraft server status.
func (s *CentralServer) handleServerStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	wrapperId := r.URL.Query().Get("wrapper")
	if wrapperId == "" {
		http.Error(w, "Wrapper ID is required", http.StatusBadRequest)
		return
	}

	wConn, exists := s.manager.GetConnection(wrapperId)
	if !exists {
		http.Error(w, "Wrapper not found", http.StatusNotFound)
		return
	}

	status, err := wConn.GetServerStatus()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(status)
}

// handleRetry handles retry requests for wrapper connections.
func (s *CentralServer) handleRetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	wrapperId := r.URL.Query().Get("wrapper")
	if wrapperId == "" {
		http.Error(w, "Wrapper ID is required", http.StatusBadRequest)
		return
	}

	wConn, exists := s.manager.GetConnection(wrapperId)
	if !exists {
		http.Error(w, "Wrapper not found", http.StatusNotFound)
		return
	}

	err := wConn.Retry()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handleWebSocket handles WebSocket connections from web clients.
func (s *CentralServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	wrapperId := r.URL.Query().Get("wrapper")
	if wrapperId == "" {
		http.Error(w, "Wrapper ID is required", http.StatusBadRequest)
		return
	}

	wConn, exists := s.manager.GetConnection(wrapperId)
	if !exists {
		http.Error(w, "Wrapper not found", http.StatusNotFound)
		return
	}

	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	// Add client to both central server and wrapper connection
	s.clientsMux.Lock()
	s.clients[ws] = true
	s.clientsMux.Unlock()
	wConn.AddClient(ws)

	// Ensure cleanup on exit
	defer func() {
		s.clientsMux.Lock()
		delete(s.clients, ws)
		s.clientsMux.Unlock()
		wConn.RemoveClient(ws)
		ws.Close()
	}()

	// Handle incoming messages from web client
	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("Web client disconnected: %v\n", err)
			}

			return
		}

		// Check if wrapper is still connected before forwarding
		if wConn.Status != StatusConnected {
			ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error: Wrapper is %s - %s", wConn.Status, wConn.Error)))

			continue
		}

		// Forward message to wrapper with timeout handling
		err = wConn.SendMessage(message)
		if err != nil {
			fmt.Printf("Error forwarding message to wrapper: %v\n", err)
			ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Error sending command: %v", err)))

			continue
		}
	}
}
