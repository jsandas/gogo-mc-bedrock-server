package downloader

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// DownloadMinecraftServer downloads and extracts the Minecraft Bedrock server
// minecraftVer is the version of the server to download (e.g. "1.20.0.01")
// appDir is the directory where the server should be extracted
// baseURL is an optional URL to download from (used for testing)
func DownloadMinecraftServer(minecraftVer string, appDir string, baseURL string) error {
	// Create temporary file for the zip
	tmpFile, err := os.CreateTemp("", "bedrock-server-*.zip")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name()) // Clean up temp file

	// Download the server
	defaultBaseURL := "https://www.minecraft.net/bedrockdedicatedserver/bin-linux"
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	url := fmt.Sprintf("%s/bedrock-server-%s.zip", baseURL, minecraftVer)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download server, status code: %d", resp.StatusCode)
	}

	// Copy the response body to the temp file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save download: %w", err)
	}

	// Ensure the temp file is closed before unzipping
	tmpFile.Close()

	// Create the app directory if it doesn't exist
	err = os.MkdirAll(appDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create app directory: %w", err)
	}

	// Extract the zip file
	zipReader, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer zipReader.Close()

	for _, file := range zipReader.File {
		err := extractFile(file, appDir)
		if err != nil {
			return fmt.Errorf("failed to extract file %s: %w", file.Name, err)
		}
	}

	return nil
}

func extractFile(file *zip.File, destDir string) error {
	// Create the destination path
	destPath := filepath.Join(destDir, file.Name)

	// Handle directories
	if file.FileInfo().IsDir() {
		return os.MkdirAll(destPath, file.Mode())
	}

	// Create parent directories if they don't exist
	err := os.MkdirAll(filepath.Dir(destPath), 0755)
	if err != nil {
		return err
	}

	// Open the file from the zip
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	// Create the destination file
	dest, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return err
	}
	defer dest.Close()

	// Copy the contents
	_, err = io.Copy(dest, src)

	return err
}
