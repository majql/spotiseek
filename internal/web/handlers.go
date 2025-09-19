package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"spotiseek/internal/config"
	"spotiseek/internal/docker"
	"spotiseek/internal/spotify"
	"spotiseek/internal/utils"
)

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	clusters, err := config.LoadClusters()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to load clusters: %v", err))
		return
	}

	response := StatusResponse{
		Playlists: make([]PlaylistStatus, 0, len(clusters.Clusters)),
		Count:     len(clusters.Clusters),
	}

	if len(clusters.Clusters) == 0 {
		s.writeJSON(w, http.StatusOK, NewSuccessResponse(response))
		return
	}

	dockerManager, err := docker.NewManager()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create Docker manager: %v", err))
		return
	}
	defer dockerManager.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get request host for Slskd URL generation
	requestHost := utils.GetRequestHost(r)

	for _, cluster := range clusters.Clusters {
		status, err := dockerManager.GetClusterStatus(ctx, cluster.PlaylistID)
		if err != nil {
			status = "error"
		}

		playlistStatus := ClusterInfoToPlaylistStatus(cluster, status)

		// Add Slskd information if container is running
		if status == "running" {
			slskdInfo, err := utils.GetSlskdInfo(ctx, dockerManager, cluster.PlaylistID, requestHost)
			if err == nil {
				playlistStatus.SlskdInfo = slskdInfo
			}
		}

		response.Playlists = append(response.Playlists, playlistStatus)
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(response))
}

func (s *Server) handleWatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req WatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "Invalid JSON request")
		return
	}

	if req.Playlist == "" {
		s.writeError(w, http.StatusBadRequest, "Playlist ID or URL is required")
		return
	}

	// Extract playlist ID
	playlistID, err := spotify.ExtractPlaylistID(req.Playlist)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid playlist ID or URL: %v", err))
		return
	}

	// Check if already watching this playlist
	clusters, err := config.LoadClusters()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to load clusters: %v", err))
		return
	}

	for _, cluster := range clusters.Clusters {
		if cluster.PlaylistID == playlistID {
			s.writeError(w, http.StatusConflict, fmt.Sprintf("Already watching playlist %s", playlistID))
			return
		}
	}

	// Verify playlist exists
	spotifyClient := spotify.NewClient(s.config.SpotifyID, s.config.SpotifySecret)
	playlist, err := spotifyClient.GetPlaylist(playlistID)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to access playlist: %v", err))
		return
	}

	// Create Docker cluster
	dockerManager, err := docker.NewManager()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create Docker manager: %v", err))
		return
	}
	defer dockerManager.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	clusterInfo, err := dockerManager.CreateCluster(ctx, playlistID, playlist.Name, s.config, req.Backfill)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create cluster: %v", err))
		return
	}

	// Add to clusters config
	clusters.Clusters = append(clusters.Clusters, *clusterInfo)
	if err := config.SaveClusters(clusters); err != nil {
		// Log warning but don't fail the request
		fmt.Printf("Warning: failed to save cluster info: %v\n", err)
	}

	response := WatchResponse{
		PlaylistID:   playlistID,
		PlaylistName: playlist.Name,
		Message:      fmt.Sprintf("Successfully started watching playlist: %s", playlist.Name),
	}

	s.writeJSON(w, http.StatusCreated, NewSuccessResponse(response))
}

func (s *Server) handleForget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Extract playlist ID from URL path: /api/forget/{playlistId}
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 || pathParts[2] == "" {
		s.writeError(w, http.StatusBadRequest, "Playlist ID is required in URL path")
		return
	}

	playlistInput := pathParts[2]

	// Extract playlist ID (handles both IDs and URLs)
	playlistID, err := spotify.ExtractPlaylistID(playlistInput)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid playlist ID or URL: %v", err))
		return
	}

	// Load clusters
	clusters, err := config.LoadClusters()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to load clusters: %v", err))
		return
	}

	// Find cluster
	clusterIndex := -1
	for i, cluster := range clusters.Clusters {
		if cluster.PlaylistID == playlistID {
			clusterIndex = i
			break
		}
	}

	if clusterIndex == -1 {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("Not watching playlist %s", playlistID))
		return
	}

	// Destroy Docker cluster
	dockerManager, err := docker.NewManager()
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create Docker manager: %v", err))
		return
	}
	defer dockerManager.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := dockerManager.DestroyCluster(ctx, playlistID); err != nil {
		// Log warning but continue with removal from config
		fmt.Printf("Warning: failed to destroy cluster: %v\n", err)
	}

	// Remove from clusters config
	clusters.Clusters = append(clusters.Clusters[:clusterIndex], clusters.Clusters[clusterIndex+1:]...)
	if err := config.SaveClusters(clusters); err != nil {
		fmt.Printf("Warning: failed to save cluster info: %v\n", err)
	}

	response := ForgetResponse{
		PlaylistID: playlistID,
		Message:    fmt.Sprintf("Successfully stopped watching playlist: %s", playlistID),
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(response))
}
