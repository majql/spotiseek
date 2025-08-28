package slskd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"spotiseek/pkg/models"
)

type Client struct {
	baseURL    string
	apiKey     string
	token      string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		apiKey:     "default-api-key", // We'll use a default key for now
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Login performs session-based authentication with Slskd
func (c *Client) Login(username, password string) error {
	loginRequest := map[string]string{
		"username": username,
		"password": password,
	}

	resp, err := c.makeRequest("POST", "/api/v0/session", loginRequest)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		// Parse the JWT token from the response
		var loginResponse struct {
			Token string `json:"token"`
		}
		
		if err := json.NewDecoder(resp.Body).Decode(&loginResponse); err != nil {
			return fmt.Errorf("failed to parse login response: %w", err)
		}
		
		c.token = loginResponse.Token
		log.Printf("Successfully logged in to Slskd and obtained JWT token")
		return nil
	}

	return fmt.Errorf("login failed with status %d", resp.StatusCode)
}

func (c *Client) makeRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	url := c.baseURL + endpoint
	req, err := http.NewRequest(method, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	
	// Use JWT token authentication if available, otherwise fall back to basic auth for login
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	} else if endpoint != "/api/v0/session" {
		// For non-session endpoints without a token, use basic auth as fallback
		req.SetBasicAuth("slskd", "slskd")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to %s: %w", url, err)
	}

	return resp, nil
}

// WaitForConnection waits for Slskd to be available with exponential backoff
func (c *Client) WaitForConnection(maxAttempts int) error {
	backoff := time.Second

	// Try different endpoints that might work
	endpoints := []string{
		"/api/v0/application",
		"/api/v0/application/state", 
		"/api/v0/server/state",
		"/api/v0/session",
		"/swagger",
		"/",
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			log.Printf("Waiting %v before retry attempt %d/%d", backoff, attempt, maxAttempts)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}

		// Try each endpoint to find one that works
		for _, endpoint := range endpoints {
			resp, err := c.makeRequest("GET", endpoint, nil)
			if err != nil {
				log.Printf("Connection attempt %d/%d failed on %s: %v", attempt, maxAttempts, endpoint, err)
				continue
			}
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				log.Printf("Successfully connected to Slskd on attempt %d using endpoint %s", attempt, endpoint)
				return nil
			}

			log.Printf("Connection attempt %d/%d got status %d on %s", attempt, maxAttempts, resp.StatusCode, endpoint)
			
			// If we get 401/403, that means the service is running but needs auth
			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				log.Printf("Slskd is running but requires authentication (status %d)", resp.StatusCode)
				return nil
			}
		}
	}

	return fmt.Errorf("failed to connect to Slskd after %d attempts", maxAttempts)
}

// CheckSoulseekConnection checks if Slskd is connected to the Soulseek network
func (c *Client) CheckSoulseekConnection() error {
	// Try multiple endpoints to check connection status
	endpoints := []string{
		"/api/v0/server/state",
		"/api/v0/server", 
		"/api/v0/application/state",
		"/api/v0/application",
		"/metrics",
	}

	for _, endpoint := range endpoints {
		resp, err := c.makeRequest("GET", endpoint, nil)
		if err != nil {
			log.Printf("Connection check failed on %s: %v", endpoint, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			log.Printf("Soulseek connection verified via %s", endpoint)
			return nil
		}
		
		log.Printf("Connection check got status %d on %s", resp.StatusCode, endpoint)
	}

	// Don't fail - just warn
	log.Printf("Warning: Unable to verify Soulseek connection via any endpoint, but continuing anyway")
	return nil
}

// Search initiates a search and returns the search ID
func (c *Client) Search(query string) (string, error) {
	searchRequest := map[string]interface{}{
		"searchText": query,
		"timeout":    30000, // 30 seconds
	}

	resp, err := c.makeRequest("POST", "/api/v0/searches", searchRequest)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("search request failed with status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode search response: %w", err)
	}

	searchID, ok := result["id"].(string)
	if !ok {
		return "", fmt.Errorf("search response missing or invalid ID")
	}

	log.Printf("Started search with ID %s for query: %s", searchID, query)
	return searchID, nil
}

// GetSearchStatus checks if a search is complete and returns status
func (c *Client) GetSearchStatus(searchID string) (*models.SearchStatus, error) {
	resp, err := c.makeRequest("GET", fmt.Sprintf("/api/v0/searches/%s", searchID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get search status failed with status %d", resp.StatusCode)
	}

	var status models.SearchStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode search status: %w", err)
	}

	return &status, nil
}

// WaitForSearchComplete waits for search to complete with polling
func (c *Client) WaitForSearchComplete(searchID string, timeout time.Duration) (*models.SearchStatus, error) {
	deadline := time.Now().Add(timeout)
	pollInterval := 2 * time.Second

	for time.Now().Before(deadline) {
		status, err := c.GetSearchStatus(searchID)
		if err != nil {
			return nil, err
		}

		// Check if search is completed (either successfully or timed out with results)
		// Handle both simple states and compound states like "Completed, TimedOut"
		if status.State == "Completed" || status.Completed || 
		   status.State == "TimedOut" || strings.Contains(status.State, "Completed") || strings.Contains(status.State, "TimedOut") {
			log.Printf("Search %s completed (state: %s) with %d results", searchID, status.State, len(status.Results))
			return status, nil
		}

		// Only treat cancelled searches as actual failures
		if status.State == "Cancelled" {
			return status, fmt.Errorf("search %s was cancelled", searchID)
		}

		log.Printf("Search %s still running (state: %s), waiting %v...", searchID, status.State, pollInterval)
		time.Sleep(pollInterval)
	}

	return nil, fmt.Errorf("search %s timed out after %v", searchID, timeout)
}

// GetSearchResults retrieves search results
func (c *Client) GetSearchResults(searchID string) ([]models.SearchResult, error) {
	resp, err := c.makeRequest("GET", fmt.Sprintf("/api/v0/searches/%s/responses", searchID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get search results failed with status %d", resp.StatusCode)
	}

	var responses []struct {
		Username string `json:"username"`
		Files    []struct {
			Filename string `json:"filename"`
			Size     int64  `json:"size"`
			Speed    int    `json:"speed"`
		} `json:"files"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&responses); err != nil {
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}

	var results []models.SearchResult
	for _, response := range responses {
		for _, file := range response.Files {
			result := models.SearchResult{
				Username: response.Username,
				Filename: file.Filename,
				Size:     file.Size,
				Speed:    file.Speed,
			}
			results = append(results, result)
		}
	}

	return results, nil
}

// DownloadFile starts downloading a file  
func (c *Client) DownloadFile(username, filename string, size int64) error {
	// Use the correct slskd API format: array of QueueDownloadRequest
	downloadRequests := []map[string]interface{}{
		{
			"filename": filename,
			"size":     size, // Use actual file size from search results
		},
	}

	// Use the correct endpoint with username as path parameter
	endpoint := fmt.Sprintf("/api/v0/transfers/downloads/%s", username)
	resp, err := c.makeRequest("POST", endpoint, downloadRequests)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("download request failed with status %d", resp.StatusCode)
	}

	log.Printf("Started download: %s from %s", filename, username)
	return nil
}

// SearchAndDownload performs a complete search and download cycle
func (c *Client) SearchAndDownload(query string, matchFunc func([]models.SearchResult) *models.SearchResult) error {
	// Start search
	searchID, err := c.Search(query)
	if err != nil {
		return fmt.Errorf("failed to start search: %w", err)
	}

	// Wait for search to complete
	_, err = c.WaitForSearchComplete(searchID, 60*time.Second)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	// Get detailed results
	results, err := c.GetSearchResults(searchID)
	if err != nil {
		return fmt.Errorf("failed to get search results: %w", err)
	}

	if len(results) == 0 {
		return fmt.Errorf("no search results found for query: %s", query)
	}

	// Find best match
	bestMatch := matchFunc(results)
	if bestMatch == nil {
		return fmt.Errorf("no suitable match found for query: %s", query)
	}

	// Start download
	if err := c.DownloadFile(bestMatch.Username, bestMatch.Filename, bestMatch.Size); err != nil {
		return fmt.Errorf("failed to start download: %w", err)
	}

	log.Printf("Successfully initiated download for query: %s", query)
	return nil
}