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

// WrapperConfig represents the configuration for a single Minecraft server wrapper
type WrapperConfig struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// Config represents the central server configuration
type Config struct {
	ListenAddress string          `json:"listen_address"`
	Wrappers      []WrapperConfig `json:"wrappers"`
}

var (
	configFile    = flag.String("config", "config.json", "path to configuration file")
	listenAddress = flag.String("listen", ":8081", "address for the web server (overrides config file)")
)

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	return &config, nil
}

func init() {
	// Set defaults from environment variables if present
	if envListenAddress := os.Getenv("LISTEN_ADDRESS"); envListenAddress != "" {
		flag.Set("listen", envListenAddress)
	}
	if envConfigFile := os.Getenv("CONFIG_FILE"); envConfigFile != "" {
		flag.Set("config", envConfigFile)
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
			if err := manager.Connect(w.ID, w.Name, w.Address, w.Username, w.Password); err != nil {
				fmt.Fprintf(os.Stderr, "Error connecting to wrapper %s (%s): %v\n", w.Name, w.ID, err)
			}
		}(wrapper)
	}

	// Create and start HTTP server
	srv := server.NewCentralServer(manager)
	serverError := make(chan error, 1)
	go func() {
		if err := srv.Start(config.ListenAddress); err != nil {
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
	if err := srv.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Error during shutdown: %v\n", err)
	}

	// Disconnect from all wrappers
	manager.DisconnectAll()

	// Wait for all wrapper connections to close
	wg.Wait()
}
