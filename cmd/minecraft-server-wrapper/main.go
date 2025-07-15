package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jsandas/gogo-mc-bedrock-server/internal/config"
	"github.com/jsandas/gogo-mc-bedrock-server/internal/runner"
	"github.com/jsandas/gogo-mc-bedrock-server/internal/server"
)

var command = flag.String("command", "./bedrock_server", "command to execute")

func init() {
	flag.Parse()
	fmt.Printf("Starting %s...\n", *command)
}

func main() {
	os.Setenv("LD_LIBRARY_PATH", ".")

	// Get the current working directory
	workDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
		os.Exit(1)
	}

	// Update server properties from environment variables
	if err := config.UpdateServerProperties(workDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error updating server properties: %v\n", err)
		os.Exit(1)
	}

	// Create command runner
	cmdRunner := runner.New(*command)

	// Start the command
	if err := cmdRunner.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting command: %v\n", err)
		os.Exit(1)
	}

	// Create and start HTTP server
	srv := server.New(cmdRunner)
	go func() {
		if err := srv.Start(":8080"); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting web server: %v\n", err)
			os.Exit(1)
		}
	}()

	// Wait for the command to complete
	if err := cmdRunner.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running command: %v\n", err)
		os.Exit(1)
	}
}
