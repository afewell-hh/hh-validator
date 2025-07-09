package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type ValidateResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Output  string `json:"output"`
	UseCase string `json:"use_case"`
	Error   string `json:"error,omitempty"`
}

var (
	wiringFile string
	fabFile    string
	serverURL  string
	verbose    bool
	timeout    int
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "validator",
		Short: "Validate Hedgehog Open Network Fabric configuration files",
		Long: `The Validator CLI tool validates ONF (Open Network Fabric) configuration files
using the hhfab utility through a web service.

Use cases:
  1. Validate wiring diagram only (generates default fab.yaml)
  2. Validate both wiring diagram and custom fab.yaml

Examples:
  # Validate wiring diagram only
  validator -w wiring.yaml

  # Validate both wiring and fabricator config
  validator -w wiring.yaml -f fab.yaml

  # Use custom server URL
  validator -w wiring.yaml -s http://remote-server:8080`,
		RunE: runValidate,
	}

	rootCmd.Flags().StringVarP(&wiringFile, "wiring", "w", "", "Path to wiring diagram file (required)")
	rootCmd.Flags().StringVarP(&fabFile, "fab", "f", "", "Path to fabricator config file (optional)")
	rootCmd.Flags().StringVarP(&serverURL, "server", "s", "http://localhost:8080", "Validator server URL")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.Flags().IntVarP(&timeout, "timeout", "t", 30, "Request timeout in seconds")

	rootCmd.MarkFlagRequired("wiring")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runValidate(cmd *cobra.Command, args []string) error {
	// Validate input files
	if err := validateInputFiles(); err != nil {
		return err
	}

	// Show configuration if verbose
	if verbose {
		fmt.Printf("Configuration:\n")
		fmt.Printf("  Wiring file: %s\n", wiringFile)
		if fabFile != "" {
			fmt.Printf("  Fab file: %s\n", fabFile)
		}
		fmt.Printf("  Server URL: %s\n", serverURL)
		fmt.Printf("  Timeout: %d seconds\n", timeout)
		fmt.Println()
	}

	// Create multipart form request
	body, contentType, err := createMultipartRequest()
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Make HTTP request
	response, err := makeRequest(body, contentType)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}

	// Display results
	displayResults(response)

	// Exit with error code if validation failed
	if !response.Success {
		os.Exit(1)
	}

	return nil
}

func validateInputFiles() error {
	// Check wiring file
	if wiringFile == "" {
		return fmt.Errorf("wiring file is required")
	}

	if _, err := os.Stat(wiringFile); os.IsNotExist(err) {
		return fmt.Errorf("wiring file does not exist: %s", wiringFile)
	}

	// Check fab file if provided
	if fabFile != "" {
		if _, err := os.Stat(fabFile); os.IsNotExist(err) {
			return fmt.Errorf("fab file does not exist: %s", fabFile)
		}
	}

	return nil
}

func createMultipartRequest() (*bytes.Buffer, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add wiring file
	if err := addFileToForm(writer, "wiring", wiringFile); err != nil {
		return nil, "", fmt.Errorf("failed to add wiring file: %w", err)
	}

	// Add fab file if provided
	if fabFile != "" {
		if err := addFileToForm(writer, "fab", fabFile); err != nil {
			return nil, "", fmt.Errorf("failed to add fab file: %w", err)
		}
	}

	// Close the multipart writer
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to close multipart writer: %w", err)
	}

	return body, writer.FormDataContentType(), nil
}

func addFileToForm(writer *multipart.Writer, fieldName, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	part, err := writer.CreateFormFile(fieldName, filepath.Base(filename))
	if err != nil {
		return err
	}

	_, err = io.Copy(part, file)
	return err
}

func makeRequest(body *bytes.Buffer, contentType string) (*ValidateResponse, error) {
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	url := strings.TrimRight(serverURL, "/") + "/validate"
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", contentType)

	if verbose {
		fmt.Printf("Making request to: %s\n", url)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response ValidateResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

func displayResults(response *ValidateResponse) {
	if response.Success {
		fmt.Printf("✓ %s\n", response.Message)
		if verbose {
			fmt.Printf("\nUse case: %s\n", response.UseCase)
			fmt.Printf("Output:\n%s\n", response.Output)
		}
	} else {
		fmt.Printf("✗ %s\n", response.Message)
		if response.Error != "" {
			fmt.Printf("Error: %s\n", response.Error)
		}
		
		if verbose && response.Output != "" {
			fmt.Printf("\nFull output:\n%s\n", response.Output)
		}
		
		if verbose {
			fmt.Printf("\nUse case: %s\n", response.UseCase)
		}
	}
}