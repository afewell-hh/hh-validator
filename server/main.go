package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type ValidateRequest struct {
	WiringFile string `form:"wiring" binding:"required"`
	FabFile    string `form:"fab"`
}

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

const (
	Version     = "1.0.0"
	MaxFileSize = 10 * 1024 * 1024 // 10MB
	TimeoutSec  = 30
)

func main() {
	// Set Gin mode from environment
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// Add request size limit middleware
	r.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxFileSize*2) // Allow for both files
		c.Next()
	})

	// Routes
	r.GET("/", getServiceInfo)
	r.GET("/health", getHealth)
	r.POST("/validate", validateFiles)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting validator server on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

func getServiceInfo(c *gin.Context) {
	response := InfoResponse{
		Service:     "ONF Validator",
		Description: "Validates Hedgehog Open Network Fabric configuration files",
		Version:     Version,
		Endpoints:   []string{"POST /validate", "GET /health", "GET /"},
	}
	c.JSON(http.StatusOK, response)
}

func getHealth(c *gin.Context) {
	// Check if hhfab is available
	if _, err := exec.LookPath("hhfab"); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  "hhfab utility not available",
		})
		return
	}

	response := HealthResponse{
		Status:    "healthy",
		Service:   "validator",
		Version:   Version,
		Timestamp: time.Now(),
	}
	c.JSON(http.StatusOK, response)
}

func validateFiles(c *gin.Context) {
	// Parse multipart form
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, ValidateResponse{
			Success: false,
			Message: "Failed to parse multipart form",
			Error:   err.Error(),
		})
		return
	}

	// Check for required wiring file
	wiringFiles := form.File["wiring"]
	if len(wiringFiles) == 0 {
		c.JSON(http.StatusBadRequest, ValidateResponse{
			Success: false,
			Message: "Missing required wiring file",
			Error:   "wiring file is required",
		})
		return
	}

	// Check for optional fab file
	fabFiles := form.File["fab"]
	var useCase string
	if len(fabFiles) > 0 {
		useCase = "uc2"
	} else {
		useCase = "uc1"
	}

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "validator-*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, ValidateResponse{
			Success: false,
			Message: "Failed to create temporary directory",
			Error:   err.Error(),
		})
		return
	}
	defer os.RemoveAll(tempDir)

	// Save wiring file to temporary location
	wiringFile := wiringFiles[0]
	wiringPath := filepath.Join(tempDir, "wiring.yaml")
	if err := c.SaveUploadedFile(wiringFile, wiringPath); err != nil {
		c.JSON(http.StatusInternalServerError, ValidateResponse{
			Success: false,
			Message: "Failed to save wiring file",
			Error:   err.Error(),
		})
		return
	}

	// Initialize hhfab command arguments
	var initArgs []string
	if useCase == "uc1" {
		// Use case 1: wiring only, generate default fab.yaml
		initArgs = []string{"init", "--dev", "-w", wiringPath}
	} else {
		// Use case 2: save fab file and use both
		fabFile := fabFiles[0]
		fabPath := filepath.Join(tempDir, "fab.yaml")
		if err := c.SaveUploadedFile(fabFile, fabPath); err != nil {
			c.JSON(http.StatusInternalServerError, ValidateResponse{
				Success: false,
				Message: "Failed to save fab file",
				Error:   err.Error(),
			})
			return
		}
		initArgs = []string{"init", "-c", fabPath, "-w", wiringPath}
	}

	// Create working directory for hhfab
	workDir := filepath.Join(tempDir, "work")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, ValidateResponse{
			Success: false,
			Message: "Failed to create work directory",
			Error:   err.Error(),
		})
		return
	}

	// Run hhfab init
	initCmd := exec.Command("hhfab", initArgs...)
	initCmd.Dir = workDir
	initOutput, err := initCmd.CombinedOutput()
	if err != nil {
		c.JSON(http.StatusBadRequest, ValidateResponse{
			Success: false,
			Message: "Failed to initialize hhfab",
			Error:   fmt.Sprintf("hhfab init failed: %s", err.Error()),
			Output:  string(initOutput),
			UseCase: useCase,
		})
		return
	}

	// Run hhfab validate
	validateCmd := exec.Command("hhfab", "validate")
	validateCmd.Dir = workDir
	validateOutput, err := validateCmd.CombinedOutput()
	
	outputStr := string(validateOutput)
	
	if err != nil {
		// Check if it's a validation error (expected) vs system error
		if strings.Contains(outputStr, "ERR") {
			c.JSON(http.StatusBadRequest, ValidateResponse{
				Success: false,
				Message: "Validation failed",
				Error:   extractErrorMessage(outputStr),
				Output:  outputStr,
				UseCase: useCase,
			})
		} else {
			c.JSON(http.StatusInternalServerError, ValidateResponse{
				Success: false,
				Message: "Failed to run validation",
				Error:   err.Error(),
				Output:  outputStr,
				UseCase: useCase,
			})
		}
		return
	}

	// Success
	c.JSON(http.StatusOK, ValidateResponse{
		Success: true,
		Message: "Fabricator config and wiring are valid",
		Output:  outputStr,
		UseCase: useCase,
	})
}

func extractErrorMessage(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "ERR") {
			// Extract the error message after "ERR"
			if idx := strings.Index(line, "ERR "); idx != -1 {
				return strings.TrimSpace(line[idx+4:])
			}
		}
	}
	return "Unknown validation error"
}