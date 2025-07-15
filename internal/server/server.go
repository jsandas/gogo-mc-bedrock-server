package server

import (
	"fmt"
	"html/template"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/jsandas/gogo-mc-bedrock-server/internal/runner"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now, should be configured in production
	},
}

// Server handles the HTTP endpoints and web UI
type Server struct {
	runner       *runner.Runner
	connections  map[*websocket.Conn]bool
	connLock     sync.RWMutex
	outputBuffer []string
}

// New creates a new Server instance
func New(runner *runner.Runner) *Server {
	srv := &Server{
		runner:      runner,
		connections: make(map[*websocket.Conn]bool),
	}

	// Start goroutine to handle runner output
	go srv.handleRunnerOutput()

	return srv
}

// Start begins the HTTP server
func (s *Server) Start(addr string) error {
	http.HandleFunc("/", s.handleIndex)
	http.HandleFunc("/ws", s.handleWebSocket)

	fmt.Printf("Web server started at http://%s\n", addr)
	return http.ListenAndServe(addr, nil)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("Error upgrading to WebSocket: %v\n", err)
		return
	}
	defer conn.Close()

	// Register connection
	s.connLock.Lock()
	s.connections[conn] = true
	s.connLock.Unlock()

	// Clean up on disconnect
	defer func() {
		s.connLock.Lock()
		delete(s.connections, conn)
		s.connLock.Unlock()
	}()

	// Send initial buffer
	s.connLock.RLock()
	for _, line := range s.outputBuffer {
		err := conn.WriteMessage(websocket.TextMessage, []byte(line))
		if err != nil {
			s.connLock.RUnlock()
			return
		}
	}
	s.connLock.RUnlock()

	// Handle incoming messages (stdin)
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}
		s.runner.WriteInput(string(message))
	}
}

func (s *Server) handleRunnerOutput() {
	for line := range s.runner.GetOutputChan() {
		// Store in buffer
		s.connLock.Lock()
		s.outputBuffer = append(s.outputBuffer, line)
		// Keep buffer size reasonable
		if len(s.outputBuffer) > 1000 {
			s.outputBuffer = s.outputBuffer[len(s.outputBuffer)-1000:]
		}
		s.connLock.Unlock()

		// Broadcast to all connections
		s.connLock.RLock()
		for conn := range s.connections {
			err := conn.WriteMessage(websocket.TextMessage, []byte(line))
			if err != nil {
				conn.Close()
				delete(s.connections, conn)
			}
		}
		s.connLock.RUnlock()
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("index").Parse(htmlTemplate))
	tmpl.Execute(w, nil)
}

const htmlTemplate = `
<!DOCTYPE html>
<html>
<head>
    <title>Minecraft Server Output</title>
    <style>
        body {
            font-family: monospace;
            background: #1e1e1e;
            color: #d4d4d4;
            padding: 20px;
        }
        #output {
            white-space: pre-wrap;
            padding: 10px;
            background: #2d2d2d;
            border-radius: 5px;
            margin-bottom: 20px;
            height: 400px;
            overflow-y: auto;
        }
        .stdout { color: #6A9955; }
        .stderr { color: #F44747; }
        .disconnected { color: #F44747; font-style: italic; }
        #input-container {
            display: flex;
            gap: 10px;
        }
        #command-input {
            flex-grow: 1;
            padding: 8px;
            background: #2d2d2d;
            border: 1px solid #3d3d3d;
            border-radius: 4px;
            color: #d4d4d4;
            font-family: monospace;
        }
        button {
            padding: 8px 16px;
            background: #0e639c;
            border: none;
            border-radius: 4px;
            color: white;
            cursor: pointer;
        }
        button:hover {
            background: #1177bb;
        }
        .status {
            position: fixed;
            top: 10px;
            right: 10px;
            padding: 5px 10px;
            border-radius: 4px;
            font-size: 12px;
        }
        .status.connected { background: #6A9955; }
        .status.disconnected { background: #F44747; }
    </style>
    <script>
        let ws;
        let reconnectAttempts = 0;
        const maxReconnectAttempts = 5;

        function connect() {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            ws = new WebSocket(protocol + '//' + window.location.host + '/ws');

            ws.onopen = function() {
                console.log('Connected to server');
                const status = document.getElementById('status');
                status.textContent = 'Connected';
                status.className = 'status connected';
                reconnectAttempts = 0;
            };

            ws.onclose = function() {
                console.log('Disconnected from server');
                const status = document.getElementById('status');
                status.textContent = 'Disconnected';
                status.className = 'status disconnected';
                if (reconnectAttempts < maxReconnectAttempts) {
                    reconnectAttempts++;
                    setTimeout(connect, 1000 * reconnectAttempts);
                } else {
                    const output = document.getElementById('output');
                    output.innerHTML += '<div class="disconnected">Connection lost. Please refresh the page to reconnect.</div>';
                }
            };

            ws.onmessage = function(event) {
                const line = event.data;
                const output = document.getElementById('output');
                const div = document.createElement('div');
                div.className = line.startsWith('[ERR]') ? 'stderr' : 'stdout';
                div.textContent = line;
                output.appendChild(div);
                output.scrollTop = output.scrollHeight;
            };

            ws.onerror = function(error) {
                console.error('WebSocket error:', error);
            };
        }

        function sendCommand() {
            const input = document.getElementById('command-input');
            const command = input.value;
            if (command.trim() === '' || !ws || ws.readyState !== WebSocket.OPEN) return;

            ws.send(command);
            input.value = '';
        }

        document.addEventListener('DOMContentLoaded', function() {
            const input = document.getElementById('command-input');
            input.addEventListener('keypress', function(e) {
                if (e.key === 'Enter') {
                    e.preventDefault();
                    sendCommand();
                }
            });
            connect();
        });
    </script>
</head>
<body>
    <div id="status" class="status disconnected">Disconnected</div>
    <h1>Minecraft Server Output</h1>
    <div id="output"></div>
    <div id="input-container">
        <input type="text" id="command-input" placeholder="Type a command and press Enter">
        <button onclick="sendCommand()">Send</button>
    </div>
</body>
</html>
`
