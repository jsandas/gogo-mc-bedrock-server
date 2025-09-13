package downloader

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadMinecraftServer(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create test files content
	testFiles := map[string][]byte{
		"bedrock_server":         []byte("#!/bin/sh\necho 'Mock server'\n"),
		"server.properties":      []byte("server-name=Test Server\n"),
		"permissions.json":       []byte("{}\n"),
		"deeply/nested/file.txt": []byte("test content\n"),
	}

	// Create a test server that serves a zip file
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check URL path
		expectedPath := "/bedrock-server-1.20.0.01.zip"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected request to %s, got %s", expectedPath, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)

			return
		}

		// Check user agent
		if r.Header.Get("User-Agent") != "Mozilla/5.0" {
			t.Errorf("Expected User-Agent 'Mozilla/5.0', got '%s'", r.Header.Get("User-Agent"))
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		// Create a zip file in memory
		w.Header().Set("Content-Type", "application/zip")
		w.WriteHeader(http.StatusOK)

		// Create the zip file
		buffer := createTestZip(t, testFiles)
		w.Write(buffer.Bytes())
	}))
	defer ts.Close()

	// Mock version for testing
	testVer := "1.20.0.01"

	// Run the downloader with our test server
	err := DownloadMinecraftServer(testVer, tempDir, ts.URL)
	if err != nil {
		t.Fatalf("DownloadMinecraftServer failed: %v", err)
	}

	// Verify all files were extracted correctly
	for filename, expectedContent := range testFiles {
		path := filepath.Join(tempDir, filename)

		content, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("Failed to read extracted file %s: %v", filename, err)
			continue
		}

		if !bytes.Equal(content, expectedContent) {
			t.Errorf("File %s content mismatch. Expected %q, got %q", filename, expectedContent, content)
		}
	}

	// Verify server file is executable
	serverPath := filepath.Join(tempDir, "bedrock_server")

	info, err := os.Stat(serverPath)
	if err != nil {
		t.Errorf("Failed to stat server file: %v", err)
	} else if info.Mode()&0111 == 0 {
		t.Error("Server file is not executable")
	}
}

// createTestZip creates a zip file in memory with the given files
func createTestZip(t *testing.T, files map[string][]byte) *bytes.Buffer {
	buffer := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buffer)

	for name, content := range files {
		// Create a file header with appropriate permissions
		header := &zip.FileHeader{
			Name: name,
		}

		// Set executable permissions for bedrock_server
		if name == "bedrock_server" {
			header.SetMode(0755) // rwxr-xr-x
		} else {
			header.SetMode(0644) // rw-r--r--
		}

		// Create file in zip with the header
		f, err := zipWriter.CreateHeader(header)
		if err != nil {
			t.Fatalf("Failed to create file in zip: %v", err)
		}

		_, err = f.Write(content)
		if err != nil {
			t.Fatalf("Failed to write content to zip: %v", err)
		}
	}

	err := zipWriter.Close()
	if err != nil {
		t.Fatalf("Failed to close zip writer: %v", err)
	}

	return buffer
}

func TestExtractFile(t *testing.T) {
	// This would test the extractFile function
	// Would need to create a zip.File mock and verify extraction
	t.Skip("Implementation needed")
}
