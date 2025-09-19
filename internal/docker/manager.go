package docker

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"spotiseek/pkg/models"
)

// sanitizeForFilesystem sanitizes a playlist name for safe filesystem use
func sanitizeForFilesystem(name string) string {
	// Replace problematic characters with safe alternatives
	sanitized := strings.ReplaceAll(name, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, "\\", "-")
	sanitized = strings.ReplaceAll(sanitized, ":", "-")
	sanitized = strings.ReplaceAll(sanitized, "*", "-")
	sanitized = strings.ReplaceAll(sanitized, "?", "-")
	sanitized = strings.ReplaceAll(sanitized, "\"", "-")
	sanitized = strings.ReplaceAll(sanitized, "<", "-")
	sanitized = strings.ReplaceAll(sanitized, ">", "-")
	sanitized = strings.ReplaceAll(sanitized, "|", "-")
	
	// Remove leading/trailing spaces and replace multiple spaces with single space
	sanitized = strings.TrimSpace(sanitized)
	sanitized = regexp.MustCompile(`\s+`).ReplaceAllString(sanitized, " ")
	
	// Limit length to reasonable filesystem limits
	if len(sanitized) > 200 {
		sanitized = sanitized[:200]
	}
	
	return sanitized
}

const (
	SlskdImage  = "slskd/slskd:latest"
	WorkerImage = "spotiseek-worker:latest"
)

type Manager struct {
	client *client.Client
}

func NewManager() (*Manager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &Manager{client: cli}, nil
}

func (m *Manager) Close() error {
	return m.client.Close()
}

func (m *Manager) pullImage(ctx context.Context, imageName string) error {
	reader, err := m.client.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", imageName, err)
	}
	defer reader.Close()

	// Must read the response stream completely
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("failed to read pull response for %s: %w", imageName, err)
	}

	return nil
}

func (m *Manager) createNetwork(ctx context.Context, networkName string) error {
	_, err := m.client.NetworkCreate(ctx, networkName, network.CreateOptions{
		Driver: "bridge",
	})
	if err != nil {
		return fmt.Errorf("failed to create network %s: %w", networkName, err)
	}
	return nil
}

func (m *Manager) removeNetwork(ctx context.Context, networkName string) error {
	return m.client.NetworkRemove(ctx, networkName)
}

func (m *Manager) createSlskdContainer(ctx context.Context, playlistID, networkName, downloadPath string) (string, error) {
	containerName := fmt.Sprintf("spotiseek-%s-slskd", playlistID)

	// Ensure download directory exists
	if err := os.MkdirAll(downloadPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create download directory: %w", err)
	}

	// Create slskd config directory
	configPath := filepath.Join(downloadPath, "slskd-config")
	if err := os.MkdirAll(configPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create slskd config directory: %w", err)
	}

	exposedPorts := nat.PortSet{
		"5030/tcp": struct{}{},
		"50300/tcp": struct{}{},
	}

	portBindings := nat.PortMap{
		"5030/tcp": []nat.PortBinding{{HostPort: "0"}}, // Random port for web interface
		"50300/tcp": []nat.PortBinding{{HostPort: "0"}}, // Random port for Soulseek connections
	}

	containerConfig := &container.Config{
		Image:        SlskdImage,
		ExposedPorts: exposedPorts,
		Env: []string{
			"SLSKD_REMOTE_CONFIGURATION=true",
			"SLSKD_SHARED_DIR=/downloads",
			"SLSKD_NO_HTTPS=true",
			"SLSKD_WEB_AUTHENTICATION_USERNAME=slskd",
			"SLSKD_WEB_AUTHENTICATION_PASSWORD=slskd",
			"SLSKD_SWAGGER=true",
			fmt.Sprintf("SLSKD_SLSK_USERNAME=%s", config.SlskUsername),
			fmt.Sprintf("SLSKD_SLSK_PASSWORD=%s", config.SlskPassword),
			"SLSKD_SLSK_CONNECTION_TIMEOUT=30000",
			"SLSKD_SLSK_INACTIVITY_TIMEOUT=300000",
		},
	}

	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		Binds: []string{
			fmt.Sprintf("%s:/downloads", downloadPath),
			fmt.Sprintf("%s:/app", configPath),
		},
	}

	networkingConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			networkName: {
				Aliases: []string{"slskd"},
			},
		},
	}

	resp, err := m.client.ContainerCreate(ctx, containerConfig, hostConfig, networkingConfig, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to create slskd container: %w", err)
	}

	return resp.ID, nil
}

func (m *Manager) createWorkerContainer(ctx context.Context, playlistID, networkName string, config *models.Config) (string, error) {
	containerName := fmt.Sprintf("spotiseek-%s-worker", playlistID)

	containerConfig := &container.Config{
		Image: WorkerImage,
		Env: []string{
			fmt.Sprintf("SPOTIFY_ID=%s", config.SpotifyID),
			fmt.Sprintf("SPOTIFY_SECRET=%s", config.SpotifySecret),
			fmt.Sprintf("SPOTIFY_PLAYLIST_ID=%s", playlistID),
			"SLSKD_URL=http://slskd:5030",
			"POLL_INTERVAL=60",
		},
	}

	hostConfig := &container.HostConfig{}

	networkingConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			networkName: {
				Aliases: []string{"worker"},
			},
		},
	}

	resp, err := m.client.ContainerCreate(ctx, containerConfig, hostConfig, networkingConfig, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to create worker container: %w", err)
	}

	return resp.ID, nil
}

func (m *Manager) startContainer(ctx context.Context, containerID string) error {
	return m.client.ContainerStart(ctx, containerID, container.StartOptions{})
}

func (m *Manager) stopContainer(ctx context.Context, containerID string) error {
	timeout := 30
	return m.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})
}

func (m *Manager) removeContainer(ctx context.Context, containerID string) error {
	return m.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
}

func (m *Manager) findContainerByName(ctx context.Context, name string) (string, error) {
	containers, err := m.client.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return "", err
	}

	for _, container := range containers {
		for _, containerName := range container.Names {
			if strings.TrimPrefix(containerName, "/") == name {
				return container.ID, nil
			}
		}
	}

	return "", fmt.Errorf("container %s not found", name)
}

func (m *Manager) CreateCluster(ctx context.Context, playlistID string, playlistName string, config *models.Config) (*models.ClusterInfo, error) {
	networkName := fmt.Sprintf("spotiseek-%s", playlistID)
	
	// Expand working directory path
	workingDir := config.WorkingDir
	if strings.HasPrefix(workingDir, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		workingDir = filepath.Join(homeDir, workingDir[2:])
	}
	
	// Sanitize playlist name for filesystem use
	sanitizedName := sanitizeForFilesystem(playlistName)
	downloadPath := filepath.Join(workingDir, sanitizedName)

	log.Printf("Creating cluster for playlist %s", playlistID)

	// Pull required images
	log.Printf("Pulling Docker images...")
	if err := m.pullImage(ctx, SlskdImage); err != nil {
		return nil, err
	}

	// Create network
	log.Printf("Creating network %s", networkName)
	if err := m.createNetwork(ctx, networkName); err != nil {
		return nil, err
	}

	// Create slskd container
	log.Printf("Creating slskd container")
	slskdID, err := m.createSlskdContainer(ctx, playlistID, networkName, downloadPath)
	if err != nil {
		m.removeNetwork(ctx, networkName) // Cleanup
		return nil, err
	}

	// Create worker container (only if we have the image)
	var workerID string
	log.Printf("Creating worker container")
	workerID, err = m.createWorkerContainer(ctx, playlistID, networkName, config)
	if err != nil {
		log.Printf("Warning: Failed to create worker container (image may not exist): %v", err)
		log.Printf("You'll need to build the worker image first")
	}

	// Start containers
	log.Printf("Starting slskd container")
	if err := m.startContainer(ctx, slskdID); err != nil {
		m.DestroyCluster(ctx, playlistID) // Cleanup
		return nil, fmt.Errorf("failed to start slskd container: %w", err)
	}

	if workerID != "" {
		log.Printf("Starting worker container")
		if err := m.startContainer(ctx, workerID); err != nil {
			log.Printf("Warning: Failed to start worker container: %v", err)
		}
	}

	log.Printf("Cluster containers created and started:")
	log.Printf("  Network: %s", networkName)
	log.Printf("  Slskd container: %s (alias: slskd)", fmt.Sprintf("spotiseek-%s-slskd", playlistID))
	log.Printf("  Worker container: %s (alias: worker)", fmt.Sprintf("spotiseek-%s-worker", playlistID))

	clusterInfo := &models.ClusterInfo{
		PlaylistID: playlistID,
		ContainerNames: models.ContainerNames{
			Worker: fmt.Sprintf("spotiseek-%s-worker", playlistID),
			Slskd:  fmt.Sprintf("spotiseek-%s-slskd", playlistID),
		},
		NetworkName: networkName,
		CreatedAt:   time.Now(),
	}

	log.Printf("Cluster created successfully for playlist %s", playlistID)
	return clusterInfo, nil
}

func (m *Manager) DestroyCluster(ctx context.Context, playlistID string) error {
	log.Printf("Destroying cluster for playlist %s", playlistID)

	containerNames := []string{
		fmt.Sprintf("spotiseek-%s-worker", playlistID),
		fmt.Sprintf("spotiseek-%s-slskd", playlistID),
	}

	// Stop and remove containers
	for _, name := range containerNames {
		containerID, err := m.findContainerByName(ctx, name)
		if err != nil {
			log.Printf("Container %s not found, skipping", name)
			continue
		}

		log.Printf("Stopping container %s", name)
		if err := m.stopContainer(ctx, containerID); err != nil {
			log.Printf("Warning: Failed to stop container %s: %v", name, err)
		}

		log.Printf("Removing container %s", name)
		if err := m.removeContainer(ctx, containerID); err != nil {
			log.Printf("Warning: Failed to remove container %s: %v", name, err)
		}
	}

	// Remove network
	networkName := fmt.Sprintf("spotiseek-%s", playlistID)
	log.Printf("Removing network %s", networkName)
	if err := m.removeNetwork(ctx, networkName); err != nil {
		log.Printf("Warning: Failed to remove network %s: %v", networkName, err)
	}

	log.Printf("Cluster destroyed for playlist %s", playlistID)
	return nil
}

func (m *Manager) GetClusterStatus(ctx context.Context, playlistID string) (string, error) {
	containerNames := []string{
		fmt.Sprintf("spotiseek-%s-worker", playlistID),
		fmt.Sprintf("spotiseek-%s-slskd", playlistID),
	}

	status := "running"
	for _, name := range containerNames {
		containerID, err := m.findContainerByName(ctx, name)
		if err != nil {
			return "not found", nil
		}

		inspect, err := m.client.ContainerInspect(ctx, containerID)
		if err != nil {
			return "error", err
		}

		if !inspect.State.Running {
			status = "stopped"
		}
	}

	return status, nil
}
