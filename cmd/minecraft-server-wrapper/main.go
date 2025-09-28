package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jsandas/gogo-mc-bedrock-server/internal/config"
	"github.com/jsandas/gogo-mc-bedrock-server/internal/downloader"
	"github.com/jsandas/gogo-mc-bedrock-server/internal/runner"
	"github.com/jsandas/gogo-mc-bedrock-server/internal/server"
)

var (
	command       = flag.String("command", "./bedrock_server", "command to execute (used for debugging purposes)")
	listenAddress = flag.String("listen", ":8080", "address for the web server")
	appDir        = flag.String("app-dir", "", "directory containing the minecraft server (defaults to current directory)")
	mcVersion     = flag.String("mc-version", "", "Minecraft version to download (if not already present)")
	authKey       = flag.String("auth-key", "", "pre-shared key for authentication (use AUTH_KEY env var instead)")
)

func init() {
	// Set defaults from environment variables if present
	if envListenAddress := os.Getenv("LISTEN_ADDRESS"); envListenAddress != "" {
		err := flag.Set("listen", envListenAddress)
		if err != nil {
			fmt.Printf("Error setting listen flag: %v\n", err)
		}
	}

	if envAppDir := os.Getenv("APP_DIR"); envAppDir != "" {
		err := flag.Set("app-dir", envAppDir)
		if err != nil {
			fmt.Printf("Error setting app-dir flag: %v\n", err)
		}
	}

	if envMcVer := os.Getenv("MINECRAFT_VER"); envMcVer != "" {
		err := flag.Set("mc-version", envMcVer)
		if err != nil {
			fmt.Printf("Error setting mc-version flag: %v\n", err)
		}
	}

	if envAuthKey := os.Getenv("AUTH_KEY"); envAuthKey != "" {
		err := flag.Set("auth-key", envAuthKey)
		if err != nil {
			fmt.Printf("Error setting auth-key flag: %v\n", err)
		}
	}

	flag.Parse()

	// Ensure we have an auth key
	if *authKey == "" {
		fmt.Fprintf(os.Stderr, "Error: Authentication key is required.\n")
		fmt.Fprintf(os.Stderr, "       Set it using the AUTH_KEY environment variable or --auth-key flag\n")
		os.Exit(1)
	}
}

func main() {
	_ = os.Setenv("LD_LIBRARY_PATH", ".")

	// Check if EULA_ACCEPT is set to true
	if eula := os.Getenv("EULA_ACCEPT"); eula != "true" {
		fmt.Fprintf(os.Stderr, "You must accept the EULA by setting EULA_ACCEPT to 'true'\n Links:\n")
		fmt.Fprintf(os.Stderr, "   https://minecraft.net/eula\n")
		fmt.Fprintf(os.Stderr, "   https://go.microsoft.com/fwlink/?LinkId=521839\n")
		os.Exit(1)
	}

	if *mcVersion == "" {
		fmt.Fprintf(os.Stderr, "Error: Minecraft version is required.\n")
		fmt.Fprintf(os.Stderr, "       Set it using the MINECRAFT_VER environment variable or --mc-version flag\n")
		os.Exit(1)
	}

	// Get the working directory
	var workDir string
	if *appDir != "" {
		workDir = *appDir
	} else {
		var err error

		workDir, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
			os.Exit(1)
		}
	}

	// Download server
	fmt.Printf("Downloading Minecraft server version %s...\n", *mcVersion)

	err := downloader.DownloadMinecraftServer(*mcVersion, workDir, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error downloading server: %v\n", err)
		os.Exit(1)
	}

	// Update server properties from environment variables
	err = config.UpdateServerProperties(workDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error updating server properties: %v\n", err)
		os.Exit(1)
	}

	// Create command runner
	cmdRunner := runner.New(*command, *appDir)

	// Start the command
	err = cmdRunner.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting command: %v\n", err)
		os.Exit(1)
	}

	// Create and start HTTP server
	srv := server.New(server.ServerConfig{
		Runner:  cmdRunner,
		AuthKey: *authKey,
	})

	go func() {
		err := srv.Start(*listenAddress)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error starting web server: %v\n", err)
			os.Exit(1)
		}
	}()

	// Wait for the command to complete
	err = cmdRunner.Wait()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running command: %v\n", err)
		os.Exit(1)
	}
}
