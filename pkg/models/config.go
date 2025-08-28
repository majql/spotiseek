package models

import (
	"time"
)

type Config struct {
	SpotifyID     string `yaml:"spotify_id"`
	SpotifySecret string `yaml:"spotify_secret"`
	WorkingDir    string `yaml:"working_dir"`
}

type ClusterInfo struct {
	PlaylistID     string            `yaml:"playlist_id"`
	ContainerNames ContainerNames    `yaml:"container_names"`
	NetworkName    string            `yaml:"network_name"`
	CreatedAt      time.Time         `yaml:"created_at"`
}

type ContainerNames struct {
	Worker string `yaml:"worker"`
	Slskd  string `yaml:"slskd"`
}

type ClustersConfig struct {
	Clusters []ClusterInfo `yaml:"clusters"`
}

type WorkerConfig struct {
	SpotifyID     string
	SpotifySecret string
	PlaylistID    string
	SlskdURL      string
	Interval      time.Duration
}