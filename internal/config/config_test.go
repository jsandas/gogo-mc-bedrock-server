package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateServerProperties(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a test server.properties file
	propsContent := `# Minecraft server properties
server-name=Dedicated Server
gamemode=survival
difficulty=normal
allow-cheats=false
max-players=10
server-port=19132
server-portv6=19133
`

	propsFile := filepath.Join(tempDir, "server.properties")
	if err := os.WriteFile(propsFile, []byte(propsContent), 0644); err != nil {
		t.Fatalf("Failed to create test properties file: %v", err)
	}

	// Set test environment variables
	os.Setenv("CFG_SERVER_NAME", "Test Server")
	os.Setenv("CFG_GAMEMODE", "creative")
	os.Setenv("CFG_MAX_PLAYERS", "20")
	os.Setenv("SOME_OTHER_VAR", "should-be-ignored")

	// Ensure environment variables are cleaned up after test
	t.Cleanup(func() {
		os.Unsetenv("CFG_SERVER_NAME")
		os.Unsetenv("CFG_GAMEMODE")
		os.Unsetenv("CFG_MAX_PLAYERS")
		os.Unsetenv("SOME_OTHER_VAR")
	})

	// Run the update
	if err := UpdateServerProperties(tempDir); err != nil {
		t.Errorf("UpdateServerProperties failed: %v", err)
	}

	// Read the updated file
	content, err := os.ReadFile(propsFile)
	if err != nil {
		t.Fatalf("Failed to read updated properties file: %v", err)
	}

	// Check if the changes were applied correctly
	updatedContent := string(content)
	expectedValues := map[string]string{
		"server-name=Test Server": "",
		"gamemode=creative":       "",
		"max-players=20":          "",
	}

	for expected := range expectedValues {
		if !contains(updatedContent, expected) {
			t.Errorf("Expected to find '%s' in properties file", expected)
		}
	}

	// Check that unchanged properties remain
	unchangedValues := map[string]string{
		"difficulty=normal":   "",
		"allow-cheats=false":  "",
		"server-port=19132":   "",
		"server-portv6=19133": "",
	}

	for unchanged := range unchangedValues {
		if !contains(updatedContent, unchanged) {
			t.Errorf("Expected unchanged value '%s' to remain in properties file", unchanged)
		}
	}
}

func TestUpdateServerPropertiesNoChanges(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a test server.properties file
	propsContent := `# Minecraft server properties
server-name=Dedicated Server
gamemode=survival
`

	propsFile := filepath.Join(tempDir, "server.properties")
	if err := os.WriteFile(propsFile, []byte(propsContent), 0644); err != nil {
		t.Fatalf("Failed to create test properties file: %v", err)
	}

	// Get original file info
	origInfo, err := os.Stat(propsFile)
	if err != nil {
		t.Fatalf("Failed to get original file info: %v", err)
	}

	// Run the update with no relevant environment variables
	if err := UpdateServerProperties(tempDir); err != nil {
		t.Errorf("UpdateServerProperties failed: %v", err)
	}

	// Get new file info
	newInfo, err := os.Stat(propsFile)
	if err != nil {
		t.Fatalf("Failed to get new file info: %v", err)
	}

	// Check that the file wasn't modified
	if newInfo.ModTime() != origInfo.ModTime() {
		t.Error("File was modified when it shouldn't have been")
	}
}

func contains(content, substr string) bool {
	return strings.Contains(content, substr)
}
