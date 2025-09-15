package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// createEchoScript creates a temporary script that echoes input and some test output.
func createEchoScript(t *testing.T) string {
	t.Helper()

	content := `#!/bin/sh
# Increase buffer size for stdin
if [ -n "$(command -v stdbuf)" ]; then
    exec stdbuf -i0 -o0 -e0 /bin/sh -c '
        while IFS= read -r line || [ -n "$line" ]; do
            printf "ECHO: %s\n" "$line"
            printf "ERROR: %s\n" "$line" >&2
        done
    '
else
    while IFS= read -r line || [ -n "$line" ]; do
        printf "ECHO: %s\n" "$line"
        printf "ERROR: %s\n" "$line" >&2
    done
fi
`
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "echo.sh")

	err := os.WriteFile(scriptPath, []byte(content), 0755)
	if err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	return scriptPath
}

func TestRunner_BasicIO(t *testing.T) {
	scriptPath := createEchoScript(t)

	// Create and start runner
	r := New(scriptPath)

	err := r.Start()
	if err != nil {
		t.Fatalf("Failed to start runner: %v", err)
	}

	// Give some time for the process to start
	time.Sleep(100 * time.Millisecond)

	// Test cases
	testInputs := []string{
		"hello world",
		"test input",
		"special chars: !@#$%",
	}
	// Channel to collect outputs
	outputs := make([]string, 0)
	done := make(chan struct{})

	// Start collecting outputs
	go func() {
		for output := range r.GetOutputChan() {
			outputs = append(outputs, output)
		}

		close(done)
	}()

	// Send inputs
	for _, input := range testInputs {
		r.WriteInput(input)
		// Give some time for the process to handle input
		time.Sleep(100 * time.Millisecond)
	}

	// Close stdin and wait for process to complete
	close(r.stdin)

	err = r.Wait()
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Wait for output collection to complete with timeout
	select {
	case <-done:
		// Success case
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for output collection")
	}

	// We expect each input to produce both stdout and stderr lines
	expectedCount := len(testInputs) * 2
	if len(outputs) < expectedCount {
		t.Errorf("Expected at least %d lines of output, got %d", expectedCount, len(outputs))
	}

	// Check if each input was properly echoed
	for _, input := range testInputs {
		foundStdout := false
		foundStderr := false

		for _, output := range outputs {
			if strings.Contains(output, "ECHO: "+input) {
				foundStdout = true
			}

			if strings.Contains(output, "ERROR: "+input) {
				foundStderr = true
			}
		}

		if !foundStdout {
			t.Errorf("Expected to find '%s' in stdout", input)
		}

		if !foundStderr {
			t.Errorf("Expected to find '%s' in stderr", input)
		}
	}
}

func TestRunner_LargeInput(t *testing.T) {
	scriptPath := createEchoScript(t)

	// Create and start runner
	r := New(scriptPath)

	err := r.Start()
	if err != nil {
		t.Fatalf("Failed to start runner: %v", err)
	}

	// Give some time for the process to start
	time.Sleep(100 * time.Millisecond)

	// Create a test pattern that's easier to verify
	pattern := "abcdefghijklmnopqrstuvwxyz0123456789"
	largeInput := strings.Repeat(pattern, 1000) // ~36KB of repeating pattern

	// Channel to collect outputs
	outputs := make([]string, 0)
	done := make(chan struct{})
	found := make(chan struct{})

	// Start collecting outputs
	go func() {
		defer close(done)

		for output := range r.GetOutputChan() {
			outputs = append(outputs, output)
			// Check if we found our input
			if strings.HasPrefix(output, "ECHO: ") {
				echoed := strings.TrimPrefix(output, "ECHO: ")
				if echoed == largeInput {
					close(found)
				}
			}
		}
	}()

	r.WriteInput(largeInput)

	// Close stdin and wait for process to complete
	close(r.stdin)

	err = r.Wait()
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Wait for either the matching output or timeout
	select {
	case <-found:
		// Success case - found the expected output
	case <-time.After(5 * time.Second):
		t.Errorf("Large input was not properly echoed after 5 seconds. Got %d lines of output", len(outputs))

		if len(outputs) > 0 {
			t.Logf("First output line: %s", outputs[0])

			if len(outputs) > 1 {
				t.Logf("Second output line: %s", outputs[1])
			}
		}
	}

	// Wait for output collection to complete
	select {
	case <-done:
		// Success case
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for output collection")
	}
}

func TestRunner_MultipleWriters(t *testing.T) {
	scriptPath := createEchoScript(t)

	// Create and start runner
	r := New(scriptPath)

	err := r.Start()
	if err != nil {
		t.Fatalf("Failed to start runner: %v", err)
	}

	// Give some time for the process to start
	time.Sleep(100 * time.Millisecond)

	// Channel to collect outputs
	outputs := make([]string, 0)
	done := make(chan struct{})

	// Start collecting outputs
	go func() {
		for output := range r.GetOutputChan() {
			outputs = append(outputs, output)
		}

		close(done)
	}()

	// Launch multiple goroutines writing simultaneously
	const numWriters = 10

	const numWrites = 10

	writersDone := make(chan bool)

	for i := 0; i < numWriters; i++ {
		go func(id int) {
			for j := 0; j < numWrites; j++ {
				input := fmt.Sprintf("writer-%d-write-%d", id, j)
				r.WriteInput(input)
			}

			writersDone <- true
		}(i)
	}

	// Wait for all writers to complete
	for i := 0; i < numWriters; i++ {
		<-writersDone
	}

	// Give some time for the process to handle all input
	time.Sleep(500 * time.Millisecond)

	// Close stdin and wait for process to complete
	close(r.stdin)

	err = r.Wait()
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// Wait for output collection to complete with timeout
	select {
	case <-done:
		// Success case
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for output collection")
	}

	// Verify outputs
	writesFound := make(map[string]bool)

	for _, output := range outputs {
		if strings.HasPrefix(output, "ECHO: writer-") {
			writesFound[output] = true
		}
	}

	expectedWrites := numWriters * numWrites
	if len(writesFound) < expectedWrites {
		t.Errorf("Expected %d unique writes, found %d", expectedWrites, len(writesFound))
	}
}
