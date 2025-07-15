package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// createEchoScript creates a temporary script that echoes input and some test output
func createEchoScript(t *testing.T) string {
	t.Helper()
	content := `#!/bin/sh
# Increase buffer size for stdin
if [ -n "$(command -v stdbuf)" ]; then
    exec stdbuf -i0 -o0 -e0 /bin/sh -c '
        while IFS= read -r line; do
            printf "ECHO: %s\n" "$line"
            printf "ERROR: %s\n" "$line" >&2
        done
    '
else
    while IFS= read -r line; do
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
	if err := r.Start(); err != nil {
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

	// Send inputs
	for _, input := range testInputs {
		r.WriteInput(input)
		// Give some time for the process to handle input
		time.Sleep(100 * time.Millisecond)
	}

	// Get output and verify
	outputs := r.GetOutput()

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

func TestOutputBuffer(t *testing.T) {
	buf := NewOutputBuffer(3) // Small buffer for testing

	// Test appending within capacity
	testLines := []string{"line1", "line2", "line3"}
	for _, line := range testLines {
		buf.Append(line)
	}

	lines := buf.GetLines()
	if len(lines) != 3 {
		t.Errorf("Expected buffer length of 3, got %d", len(lines))
	}

	// Test that lines match
	for i, expected := range testLines {
		if lines[i] != expected {
			t.Errorf("Expected line %d to be '%s', got '%s'", i, expected, lines[i])
		}
	}

	// Test overflow behavior
	buf.Append("line4")
	lines = buf.GetLines()

	if len(lines) != 3 {
		t.Errorf("Expected buffer length to stay at 3, got %d", len(lines))
	}

	expectedLines := []string{"line2", "line3", "line4"}
	for i, expected := range expectedLines {
		if lines[i] != expected {
			t.Errorf("Expected line %d to be '%s', got '%s'", i, expected, lines[i])
		}
	}
}

func TestRunner_LargeInput(t *testing.T) {
	scriptPath := createEchoScript(t)

	// Create and start runner
	r := New(scriptPath)
	if err := r.Start(); err != nil {
		t.Fatalf("Failed to start runner: %v", err)
	}

	// Give some time for the process to start
	time.Sleep(100 * time.Millisecond)

	// Create a test pattern that's easier to verify
	pattern := "abcdefghijklmnopqrstuvwxyz0123456789"
	largeInput := strings.Repeat(pattern, 1000) // ~36KB of repeating pattern
	r.WriteInput(largeInput)

	// Give more time for the process to handle input and produce output
	deadline := time.Now().Add(5 * time.Second)
	found := false
	var outputs []string

	for time.Now().Before(deadline) {
		outputs = r.GetOutput()
		for _, output := range outputs {
			if strings.HasPrefix(output, "ECHO: ") {
				// Verify the echoed content matches our input
				echoed := strings.TrimPrefix(output, "ECHO: ")
				if echoed == largeInput {
					found = true
					break
				}
			}
		}
		if found {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !found {
		t.Errorf("Large input was not properly echoed after 5 seconds. Got %d lines of output", len(outputs))
		if len(outputs) > 0 {
			// Print the first line to help diagnose the issue
			t.Logf("First output line: %s", outputs[0])
			if len(outputs) > 1 {
				t.Logf("Second output line: %s", outputs[1])
			}
		}
	}
}

func TestRunner_MultipleWriters(t *testing.T) {
	scriptPath := createEchoScript(t)

	// Create and start runner
	r := New(scriptPath)
	if err := r.Start(); err != nil {
		t.Fatalf("Failed to start runner: %v", err)
	}

	// Give some time for the process to start
	time.Sleep(100 * time.Millisecond)

	// Launch multiple goroutines writing simultaneously
	const numWriters = 10
	const numWrites = 10
	done := make(chan bool)

	for i := 0; i < numWriters; i++ {
		go func(id int) {
			for j := 0; j < numWrites; j++ {
				input := fmt.Sprintf("writer-%d-write-%d", id, j)
				r.WriteInput(input)
			}
			done <- true
		}(i)
	}

	// Wait for all writers to complete
	for i := 0; i < numWriters; i++ {
		<-done
	}

	// Give some time for the process to handle all input
	time.Sleep(500 * time.Millisecond)

	// Verify outputs
	outputs := r.GetOutput()
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
