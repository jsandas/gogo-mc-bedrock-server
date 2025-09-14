package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/jsandas/gogo-mc-bedrock-server/internal/server"
)

// WrapperConfig represents the configuration for a single Minecraft server wrapper.
type WrapperConfig struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Address   string `json:"address"`
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
	SharedKey string `json:"shared_key"` // Key that must match the wrapper's AUTH_KEY
}

// Config represents the central server configuration.
type Config struct {
	ListenAddress string          `json:"listen_address"`
	AuthKey       string          `json:"auth_key,omitempty"`
	Wrappers      []WrapperConfig `json:"wrappers"`
}

var (
	configFile    = flag.String("config", "config.json", "path to configuration file")
	listenAddress = flag.String("listen", ":8081", "address for the web server (overrides config file)")
	authKey       = flag.String("auth-key", "", "pre-shared key for authentication (overrides config file)")
)

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	var config Config

	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	return &config, nil
}

func init() {
	// Set defaults from environment variables if present
	if envListenAddress := os.Getenv("LISTEN_ADDRESS"); envListenAddress != "" {
		err := flag.Set("listen", envListenAddress)
		if err != nil {
			fmt.Printf("Error setting listen flag: %v\n", err)
		}
	}

	if envConfigFile := os.Getenv("CONFIG_FILE"); envConfigFile != "" {
		err := flag.Set("config", envConfigFile)
		if err != nil {
			fmt.Printf("Error setting config flag: %v\n", err)
		}
	}

	if envAuthKey := os.Getenv("AUTH_KEY"); envAuthKey != "" {
		err := flag.Set("auth-key", envAuthKey)
		if err != nil {
			fmt.Printf("Error setting auth-key flag: %v\n", err)
		}
	}

	flag.Parse()
}

func main() {
	// Load configuration
	config, err := loadConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Override listen address if provided via flag
	if *listenAddress != ":8081" {
		config.ListenAddress = *listenAddress
	}

	// Create connection manager
	manager := server.NewConnectionManager()

	// Connect to all configured wrappers
	var wg sync.WaitGroup
	for _, wrapper := range config.Wrappers {
		wg.Add(1)

		go func(w WrapperConfig) {
			defer wg.Done()
			// Ensure wrapper has a shared key configured
			if w.SharedKey == "" {
				fmt.Fprintf(os.Stderr, "Error: Wrapper %s (%s) is missing a shared_key in config\n", w.Name, w.ID)
				return
			}

			// Attempt to connect but don't fail if connection fails
			err := manager.Connect(w.ID, w.Name, w.Address, w.Username, w.Password, w.SharedKey)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Initial connection to wrapper %s (%s) failed: %v\n", w.Name, w.ID, err)
				fmt.Fprintf(os.Stderr, "Will attempt to reconnect automatically...\n")
			}
		}(wrapper)
	}

	// Determine the auth key to use (priority: env/flag > config file)
	finalAuthKey := config.AuthKey
	if *authKey != "" {
		finalAuthKey = *authKey
	}

	// Ensure we have an auth key
	if finalAuthKey == "" {
		fmt.Fprintf(os.Stderr, "Error: Authentication key is required. Set it using:\n")
		fmt.Fprintf(os.Stderr, "  1. AUTH_KEY environment variable (recommended)\n")
		fmt.Fprintf(os.Stderr, "  2. --auth-key command line flag\n")
		fmt.Fprintf(os.Stderr, "  3. auth_key field in config.json\n")
		os.Exit(1)
	}

	// Create and start HTTP server
	srv := server.NewCentralServer(server.CentralServerConfig{
		Manager: manager,
		AuthKey: finalAuthKey,
	})
	serverError := make(chan error, 1)

	go func() {
		err := srv.Start(config.ListenAddress)
		if err != nil {
			serverError <- err
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for either server error or interrupt
	select {
	case err := <-serverError:
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
	case <-sigChan:
		fmt.Println("\nReceived interrupt signal. Shutting down...")
	}

	// Graceful shutdown
	err = srv.Stop()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error during shutdown: %v\n", err)
	}

	// Disconnect from all wrappers
	manager.DisconnectAll()

	// Wait for all wrapper connections to close
	wg.Wait()
}
