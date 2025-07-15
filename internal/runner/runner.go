package runner

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
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

// Runner manages the execution of a command and its I/O
type Runner struct {
	cmd          *exec.Cmd
	outputBuffer *OutputBuffer
	stdin        chan string
}

// New creates a new Runner instance
func New(command string, args ...string) *Runner {
	return &Runner{
		cmd:          exec.Command(command, args...),
		outputBuffer: NewOutputBuffer(1000),
		stdin:        make(chan string),
	}
}

// Start begins the command execution and sets up I/O handling
func (r *Runner) Start() error {
	// Create stdin pipe
	stdin, err := r.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("error creating stdin pipe: %v", err)
	}

	// Create stdout pipe
	stdout, err := r.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating stdout pipe: %v", err)
	}

	// Create stderr pipe
	stderr, err := r.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error creating stderr pipe: %v", err)
	}

	// Start command
	if err := r.cmd.Start(); err != nil {
		return fmt.Errorf("error starting command: %v", err)
	}

	// Create scanners for stdout and stderr
	outScanner := bufio.NewScanner(stdout)
	errScanner := bufio.NewScanner(stderr)

	// Start goroutine to scan stdout
	go func() {
		for outScanner.Scan() {
			line := outScanner.Text()
			fmt.Println(line)
			r.outputBuffer.Append(line)
		}
	}()

	// Start goroutine to scan stderr
	go func() {
		for errScanner.Scan() {
			line := "[ERR] " + errScanner.Text()
			fmt.Println(line)
			r.outputBuffer.Append(line)
		}
	}()

	// Start goroutine to handle terminal input
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			r.stdin <- scanner.Text()
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading from stdin: %v\n", err)
		}
	}()

	// Start goroutine to forward input to the process
	go func() {
		for input := range r.stdin {
			input = input + "\n"
			_, err := stdin.Write([]byte(input))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error writing to stdin: %v\n", err)
				return
			}
		}
	}()

	return nil
}

// WriteInput sends input to the running command
func (r *Runner) WriteInput(input string) {
	r.stdin <- input
}

// GetOutput returns the current output buffer contents
func (r *Runner) GetOutput() []string {
	return r.outputBuffer.GetLines()
}

// Wait waits for the command to complete
func (r *Runner) Wait() error {
	return r.cmd.Wait()
}
