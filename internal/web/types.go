package web

import (
	"time"
	"spotiseek/pkg/models"
)

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type StatusResponse struct {
	Playlists []PlaylistStatus `json:"playlists"`
	Count     int              `json:"count"`
}

type PlaylistStatus struct {
	PlaylistID     string    `json:"playlist_id"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	WorkerName     string    `json:"worker_name"`
	SlskdName      string    `json:"slskd_name"`
	NetworkName    string    `json:"network_name"`
}

type WatchRequest struct {
	Playlist string `json:"playlist"`
}

type WatchResponse struct {
	PlaylistID   string `json:"playlist_id"`
	PlaylistName string `json:"playlist_name"`
	Message      string `json:"message"`
}

type ForgetResponse struct {
	PlaylistID string `json:"playlist_id"`
	Message    string `json:"message"`
}

func NewSuccessResponse(data interface{}) APIResponse {
	return APIResponse{
		Success: true,
		Data:    data,
	}
}

func NewErrorResponse(err string) APIResponse {
	return APIResponse{
		Success: false,
		Error:   err,
	}
}

func ClusterInfoToPlaylistStatus(cluster models.ClusterInfo, status string) PlaylistStatus {
	return PlaylistStatus{
		PlaylistID:  cluster.PlaylistID,
		Status:      status,
		CreatedAt:   cluster.CreatedAt,
		WorkerName:  cluster.ContainerNames.Worker,
		SlskdName:   cluster.ContainerNames.Slskd,
		NetworkName: cluster.NetworkName,
	}
}