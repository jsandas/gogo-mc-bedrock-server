package main

import (
	"bufio"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// OutputBuffer stores the last N lines of output
type OutputBuffer struct {
	mu     sync.Mutex
	lines  []string
	maxLen int
}

func NewOutputBuffer(maxLen int) *OutputBuffer {
	return &OutputBuffer{
		lines:  make([]string, 0, maxLen),
		maxLen: maxLen,
	}
}

func (b *OutputBuffer) Append(line string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.lines = append(b.lines, line)
	if len(b.lines) > b.maxLen {
		b.lines = b.lines[1:]
	}
}

func (b *OutputBuffer) GetLines() []string {
	b.mu.Lock()
	defer b.mu.Unlock()

	result := make([]string, len(b.lines))
	copy(result, b.lines)
	return result
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

func main() {
	os.Setenv("LD_LIBRARY_PATH", ".")

	// Create output buffer
	outputBuffer := NewOutputBuffer(1000)

	// Create channel for web input
	webInput := make(chan string)

	// Start HTTP server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.New("index").Parse(htmlTemplate))
		tmpl.Execute(w, nil)
	})

	http.HandleFunc("/output", func(w http.ResponseWriter, r *http.Request) {
		lines := outputBuffer.GetLines()
		for _, line := range lines {
			var class string
			if strings.HasPrefix(line, "[ERR]") {
				class = "stderr"
			} else {
				class = "stdout"
			}
			fmt.Fprintf(w, "<div class='%s'>%s</div>", class, template.HTMLEscapeString(line))
		}
	})

	http.HandleFunc("/input", func(w http.ResponseWriter, r *http.Request) {
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
			webInput <- command
		}
		w.WriteHeader(http.StatusOK)
	})

	go func() {
		fmt.Println("Web server started at http://localhost:8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting web server: %v\n", err)
			os.Exit(1)
		}
	}()

	// Create command to execute test-app
	// cmd := exec.Command("./test-app")
	cmd := exec.Command("./bedrock_server")

	// Create stdin pipe
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating stdin pipe: %v\n", err)
		os.Exit(1)
	}

	// Create stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating stdout pipe: %v\n", err)
		os.Exit(1)
	}

	// Create stderr pipe
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating stderr pipe: %v\n", err)
		os.Exit(1)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting test-app: %v\n", err)
		os.Exit(1)
	}

	// Create scanners for both stdout and stderr
	outScanner := bufio.NewScanner(stdout)
	errScanner := bufio.NewScanner(stderr)

	// Start goroutine to scan stdout
	go func() {
		for outScanner.Scan() {
			line := outScanner.Text()
			fmt.Println(line)
			outputBuffer.Append(line)
		}
	}()

	// Start goroutine to scan stderr
	go func() {
		for errScanner.Scan() {
			line := "[ERR] " + errScanner.Text()
			fmt.Println(line)
			outputBuffer.Append(line)
		}
	}()

	// Start goroutine to handle terminal input
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			input := scanner.Text()
			webInput <- input
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading from stdin: %v\n", err)
		}
	}()

	// Start goroutine to forward input to the test-app process
	go func() {
		for input := range webInput {
			input = input + "\n"
			_, err := stdin.Write([]byte(input))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error writing to stdin: %v\n", err)
				return
			}
		}
	}()

	// Wait for the command to complete
	if err := cmd.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running test-app: %v\n", err)
		os.Exit(1)
	}
}
