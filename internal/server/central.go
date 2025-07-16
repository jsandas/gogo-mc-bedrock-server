package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// CentralServer represents the central management server
type CentralServer struct {
	manager    *ConnectionManager
	server     *http.Server
	upgrader   websocket.Upgrader
	clients    map[*websocket.Conn]bool
	clientsMux sync.RWMutex
}

// NewCentralServer creates a new central server instance
func NewCentralServer(manager *ConnectionManager) *CentralServer {
	return &CentralServer{
		manager: manager,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for now
			},
		},
		clients: make(map[*websocket.Conn]bool),
	}
}

// Start starts the HTTP server
func (s *CentralServer) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/wrappers", s.handleWrappers)
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.Handle("/", http.FileServer(http.Dir("web")))

	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return s.server.ListenAndServe()
}

// Stop gracefully shuts down the server
func (s *CentralServer) Stop() error {
	return s.server.Shutdown(context.Background())
}

// handleWrappers handles requests for wrapper information
func (s *CentralServer) handleWrappers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	wrappers := s.manager.ListConnections()
	json.NewEncoder(w).Encode(wrappers)
}

// handleWebSocket handles WebSocket connections from web clients
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
				fmt.Printf("Web client error: %v\n", err)
			}
			break
		}

		// Forward message to wrapper
		if err := wConn.SendMessage(message); err != nil {
			fmt.Printf("Error forwarding message to wrapper: %v\n", err)
			break
		}
	}
}
