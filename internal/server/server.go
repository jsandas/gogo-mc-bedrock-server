package server

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/jsandas/gogo-mc-bedrock-server/internal/runner"
)

// Server handles the HTTP endpoints and web UI
type Server struct {
	runner *runner.Runner
}

// New creates a new Server instance
func New(runner *runner.Runner) *Server {
	return &Server{
		runner: runner,
	}
}

// Start begins the HTTP server
func (s *Server) Start(addr string) error {
	http.HandleFunc("/", s.handleIndex)
	http.HandleFunc("/output", s.handleOutput)
	http.HandleFunc("/input", s.handleInput)

	fmt.Printf("Web server started at http://%s\n", addr)
	return http.ListenAndServe(addr, nil)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("index").Parse(htmlTemplate))
	tmpl.Execute(w, nil)
}

func (s *Server) handleOutput(w http.ResponseWriter, r *http.Request) {
	lines := s.runner.GetOutput()
	for _, line := range lines {
		var class string
		if strings.HasPrefix(line, "[ERR]") {
			class = "stderr"
		} else {
			class = "stdout"
		}
		fmt.Fprintf(w, "<div class='%s'>%s</div>", class, template.HTMLEscapeString(line))
	}
}

func (s *Server) handleInput(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}
	command := r.FormValue("command")
	if command != "" {
		s.runner.WriteInput(command)
	}
	w.WriteHeader(http.StatusOK)
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
    </style>
    <script>
        function refreshOutput() {
            fetch('/output')
                .then(response => response.text())
                .then(html => {
                    const output = document.getElementById('output');
                    output.innerHTML = html;
                    output.scrollTop = output.scrollHeight;
                });
        }

        function sendCommand() {
            const input = document.getElementById('command-input');
            const command = input.value;
            if (command.trim() === '') return;

            fetch('/input', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/x-www-form-urlencoded',
                },
                body: 'command=' + encodeURIComponent(command)
            }).then(() => {
                input.value = '';
            });
        }

        document.addEventListener('DOMContentLoaded', function() {
            const input = document.getElementById('command-input');
            input.addEventListener('keypress', function(e) {
                if (e.key === 'Enter') {
                    sendCommand();
                }
            });
        });

        setInterval(refreshOutput, 1000);
    </script>
</head>
<body>
    <h1>Minecraft Server Output</h1>
    <div id="output"></div>
    <div id="input-container">
        <input type="text" id="command-input" placeholder="Type a command and press Enter">
        <button onclick="sendCommand()">Send</button>
    </div>
</body>
</html>
`
