package main

import (
	"fmt"
	"os"

	"github.com/jsandas/gogo-mc-bedrock-server/internal/runner"
	"github.com/jsandas/gogo-mc-bedrock-server/internal/server"
)

func main() {
	os.Setenv("LD_LIBRARY_PATH", ".")

	// Create command runner
	cmdRunner := runner.New("./bedrock_server")

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
