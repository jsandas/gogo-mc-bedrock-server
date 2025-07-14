package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
)

func main() {
	os.Setenv("LD_LIBRARY_PATH", ".")

	// Create command to execute test-app
	cmd := exec.Command("./test-app")
	// cmd := exec.Command("./bedrock_server")

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
			fmt.Printf("[OUT] %s\n", outScanner.Text())
		}
	}()

	// Start goroutine to scan stderr
	go func() {
		for errScanner.Scan() {
			fmt.Printf("[ERR] %s\n", errScanner.Text())
		}
	}()

	// Start goroutine to forward stdin to the test-app process
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			input := scanner.Text() + "\n"
			_, err := stdin.Write([]byte(input))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error writing to stdin: %v\n", err)
				return
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading from stdin: %v\n", err)
		}
	}()

	// Wait for the command to complete
	if err := cmd.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running test-app: %v\n", err)
		os.Exit(1)
	}
}
