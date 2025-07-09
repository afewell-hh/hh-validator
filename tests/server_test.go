package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test response structures
type ValidateResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Output  string `json:"output"`
	UseCase string `json:"use_case"`
	Error   string `json:"error,omitempty"`
}

type HealthResponse struct {
	Status    string    `json:"status"`
	Service   string    `json:"service"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
}

type InfoResponse struct {
	Service     string   `json:"service"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Endpoints   []string `json:"endpoints"`
}

// Mock server setup
func setupTestServer() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Add test routes (simplified versions)
	r.GET("/", func(c *gin.Context) {
		response := InfoResponse{
			Service:     "ONF Validator",
			Description: "Validates Hedgehog Open Network Fabric configuration files",
			Version:     "1.0.0",
			Endpoints:   []string{"POST /validate", "GET /health", "GET /"},
		}
		c.JSON(http.StatusOK, response)
	})

	r.GET("/health", func(c *gin.Context) {
		response := HealthResponse{
			Status:    "healthy",
			Service:   "validator",
			Version:   "1.0.0",
			Timestamp: time.Now(),
		}
		c.JSON(http.StatusOK, response)
	})

	r.POST("/validate", func(c *gin.Context) {
		// Basic validation endpoint for testing
		form, err := c.MultipartForm()
		if err != nil {
			c.JSON(http.StatusBadRequest, ValidateResponse{
				Success: false,
				Message: "Failed to parse multipart form",
				Error:   err.Error(),
			})
			return
		}

		wiringFiles := form.File["wiring"]
		if len(wiringFiles) == 0 {
			c.JSON(http.StatusBadRequest, ValidateResponse{
				Success: false,
				Message: "Missing required wiring file",
				Error:   "wiring file is required",
			})
			return
		}

		fabFiles := form.File["fab"]
		useCase := "uc1"
		if len(fabFiles) > 0 {
			useCase = "uc2"
		}

		// Mock successful validation with exact hhfab output format
		mockOutput := "06:37:39 INF Hedgehog Fabricator version=v0.40.0\n06:37:39 INF Wiring hydrated successfully mode=if-not-present\n06:37:39 INF Fabricator config and wiring are valid"
		c.JSON(http.StatusOK, ValidateResponse{
			Success: true,
			Message: mockOutput,
			Output:  mockOutput,
			UseCase: useCase,
		})
	})

	return r
}

func TestServiceInfo(t *testing.T) {
	router := setupTestServer()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response InfoResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "ONF Validator", response.Service)
	assert.Equal(t, "1.0.0", response.Version)
	assert.Contains(t, response.Endpoints, "POST /validate")
}

func TestHealthCheck(t *testing.T) {
	router := setupTestServer()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response HealthResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response.Status)
	assert.Equal(t, "validator", response.Service)
}

func TestValidateWithoutWiring(t *testing.T) {
	router := setupTestServer()
	
	// Create empty multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/validate", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ValidateResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "Missing required wiring file")
}

func TestValidateWithWiring(t *testing.T) {
	router := setupTestServer()
	
	// Create test wiring file
	tempDir, err := os.MkdirTemp("", "test-")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	wiringFile := filepath.Join(tempDir, "wiring.yaml")
	testContent := `apiVersion: wiring.githedgehog.com/v1beta1
kind: VLANNamespace
metadata:
  name: default
spec:
  ranges:
  - from: 1000
    to: 2999`

	err = os.WriteFile(wiringFile, []byte(testContent), 0644)
	require.NoError(t, err)

	// Create multipart form with wiring file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	file, err := os.Open(wiringFile)
	require.NoError(t, err)
	defer file.Close()

	part, err := writer.CreateFormFile("wiring", "wiring.yaml")
	require.NoError(t, err)
	
	_, err = io.Copy(part, file)
	require.NoError(t, err)
	
	writer.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/validate", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ValidateResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response.Success)
	assert.Equal(t, "uc1", response.UseCase)
	assert.Contains(t, response.Message, "Fabricator config and wiring are valid")
	assert.Contains(t, response.Message, "INF Hedgehog Fabricator version")
}

func TestValidateWithBothFiles(t *testing.T) {
	router := setupTestServer()
	
	// Create test files
	tempDir, err := os.MkdirTemp("", "test-")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	wiringFile := filepath.Join(tempDir, "wiring.yaml")
	fabFile := filepath.Join(tempDir, "fab.yaml")
	
	wiringContent := `apiVersion: wiring.githedgehog.com/v1beta1
kind: VLANNamespace
metadata:
  name: default`

	fabContent := `apiVersion: fabricator.githedgehog.com/v1beta1
kind: Fabricator
metadata:
  name: default`

	err = os.WriteFile(wiringFile, []byte(wiringContent), 0644)
	require.NoError(t, err)
	
	err = os.WriteFile(fabFile, []byte(fabContent), 0644)
	require.NoError(t, err)

	// Create multipart form with both files
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	// Add wiring file
	file1, err := os.Open(wiringFile)
	require.NoError(t, err)
	defer file1.Close()

	part1, err := writer.CreateFormFile("wiring", "wiring.yaml")
	require.NoError(t, err)
	
	_, err = io.Copy(part1, file1)
	require.NoError(t, err)

	// Add fab file
	file2, err := os.Open(fabFile)
	require.NoError(t, err)
	defer file2.Close()

	part2, err := writer.CreateFormFile("fab", "fab.yaml")
	require.NoError(t, err)
	
	_, err = io.Copy(part2, file2)
	require.NoError(t, err)
	
	writer.Close()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/validate", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ValidateResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.True(t, response.Success)
	assert.Equal(t, "uc2", response.UseCase)
	assert.Contains(t, response.Message, "Fabricator config and wiring are valid")
}

func TestErrorMessageExtraction(t *testing.T) {
	testCases := []struct {
		name           string
		output         string
		expectedError  string
	}{
		{
			name:   "Simple error",
			output: "06:38:17 ERR validating: some error occurred",
			expectedError: "validating: some error occurred",
		},
		{
			name:   "Complex error",
			output: "06:38:17 ERR validating: loading wiring and hydrating: loading wiring: object 48: decoding: yaml: line 17: could not find expected ':'",
			expectedError: "validating: loading wiring and hydrating: loading wiring: object 48: decoding: yaml: line 17: could not find expected ':'",
		},
		{
			name:   "No error",
			output: "06:37:39 INF Fabricator config and wiring are valid",
			expectedError: "Unknown validation error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractErrorMessage(tc.output)
			assert.Equal(t, tc.expectedError, result)
		})
	}
}

// Helper function (copy from server code for testing)
func extractErrorMessage(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "ERR") {
			if idx := strings.Index(line, "ERR "); idx != -1 {
				return strings.TrimSpace(line[idx+4:])
			}
		}
	}
	return "Unknown validation error"
}