package utils

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"spotiseek/internal/docker"
)

const (
	SlskdUsername = "slskd"
	SlskdPassword = "slskd"
)

// GenerateSlskdURL creates a Slskd URL with auto-login for the given playlist
func GenerateSlskdURL(ctx context.Context, dockerManager *docker.Manager, playlistID, requestHost string) (string, error) {
	port, err := dockerManager.GetSlskdPort(ctx, playlistID)
	if err != nil {
		return "", fmt.Errorf("failed to get slskd port: %w", err)
	}

	// Determine the base URL from the request host
	baseURL := "http://localhost"
	if requestHost != "" && requestHost != "localhost" {
		// Use the same host the user came from
		if strings.Contains(requestHost, ":") {
			// Remove any existing port from host
			hostParts := strings.Split(requestHost, ":")
			baseURL = fmt.Sprintf("http://%s", hostParts[0])
		} else {
			baseURL = fmt.Sprintf("http://%s", requestHost)
		}
	}

	// Create the Slskd URL with auto-login
	slskdURL := fmt.Sprintf("%s:%s", baseURL, port)

	// For web interface, we'll return the base URL
	return slskdURL, nil
}

// GetSlskdInfo returns comprehensive Slskd connection information
func GetSlskdInfo(ctx context.Context, dockerManager *docker.Manager, playlistID, requestHost string) (*SlskdInfo, error) {
	port, err := dockerManager.GetSlskdPort(ctx, playlistID)
	if err != nil {
		return nil, fmt.Errorf("failed to get slskd port: %w", err)
	}

	// Determine the base URL from the request host
	baseURL := "http://localhost"
	if requestHost != "" && requestHost != "localhost" {
		// Use the same host the user came from
		if strings.Contains(requestHost, ":") {
			// Remove any existing port from host
			hostParts := strings.Split(requestHost, ":")
			baseURL = fmt.Sprintf("http://%s", hostParts[0])
		} else {
			baseURL = fmt.Sprintf("http://%s", requestHost)
		}
	}

	slskdURL := fmt.Sprintf("%s:%s", baseURL, port)

	return &SlskdInfo{
		URL:      slskdURL,
		Port:     port,
		Username: SlskdUsername,
		Password: SlskdPassword,
		LoginURL: fmt.Sprintf("%s/api/v0/session", slskdURL),
	}, nil
}

// SlskdInfo contains all information needed to connect to Slskd
type SlskdInfo struct {
	URL      string `json:"url"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	LoginURL string `json:"login_url"`
}

// GetRequestHost extracts the host from an HTTP request
func GetRequestHost(r *http.Request) string {
	if r == nil {
		return ""
	}

	// Check X-Forwarded-Host header first (for proxies)
	if host := r.Header.Get("X-Forwarded-Host"); host != "" {
		return host
	}

	// Use Host header
	return r.Host
}
