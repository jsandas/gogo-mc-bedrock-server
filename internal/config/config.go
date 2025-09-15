package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UpdateServerProperties reads environment variables prefixed with CFG_ and updates
// the server.properties file accordingly.
func UpdateServerProperties(appDir string) error {
	propsFile := filepath.Join(appDir, "server.properties")

	// Get all relevant environment variables first
	envVars := make(map[string]string)

	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "CFG_") {
			continue
		}

		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		// Remove CFG_ prefix and convert _ to -
		key := strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(parts[0], "CFG_"), "_", "-"))
		value := parts[1]
		envVars[key] = value
	}

	// Don't even open the file if there are no variables to process
	if len(envVars) == 0 {
		return nil
	}

	// Read the current server.properties file
	lines, err := readPropertiesFile(propsFile)
	if err != nil {
		return fmt.Errorf("error reading properties file: %v", err)
	}

	// Update the properties
	updated := false
	newLines := make([]string, len(lines))
	copy(newLines, lines)

	for i, line := range newLines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		if newValue, exists := envVars[key]; exists {
			currentValue := strings.TrimSpace(parts[1])
			if currentValue != newValue {
				newLines[i] = fmt.Sprintf("%s=%s", key, newValue)
				updated = true

				fmt.Printf("Updating %s from %s to %s\n", key, currentValue, newValue)
			}
		}
	}

	// Only write the file if we found actual changes
	if updated {
		err := writePropertiesFile(propsFile, newLines)
		if err != nil {
			return fmt.Errorf("error writing properties file: %v", err)
		}
	}

	return nil
}

func readPropertiesFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath) // #nosec G304
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	err = scanner.Err()
	if err != nil {
		return nil, err
	}

	return lines, nil
}

func writePropertiesFile(filePath string, lines []string) error {
	file, err := os.Create(filePath) // #nosec G304
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, line := range lines {
		_, err := writer.WriteString(line + "\n")
		if err != nil {
			return err
		}
	}

	return writer.Flush()
}
