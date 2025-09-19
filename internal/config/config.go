package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"spotiseek/pkg/models"
)

const (
	DefaultWorkingDir = "~/spotiseek"
	ConfigDir         = ".spotiseek"
	ConfigFile        = "spotiseek.yml"
	ClustersFile      = "clusters.yml"
)

func GetConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ConfigDir), nil
}

func EnsureConfigDir() error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(configDir, 0755)
}

func LoadConfig() (*models.Config, error) {
	if err := EnsureConfigDir(); err != nil {
		return nil, err
	}

	configDir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(configDir, ConfigFile)
	config := &models.Config{}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config if it doesn't exist
		config.WorkingDir = DefaultWorkingDir
		return config, SaveConfig(config)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if config.WorkingDir == "" {
		config.WorkingDir = DefaultWorkingDir
	}

	return config, nil
}

func SaveConfig(config *models.Config) error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	configPath := filepath.Join(configDir, ConfigFile)
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(configPath, data, 0644)
}

func LoadClusters() (*models.ClustersConfig, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}

	clustersPath := filepath.Join(configDir, ClustersFile)
	clusters := &models.ClustersConfig{}

	if _, err := os.Stat(clustersPath); os.IsNotExist(err) {
		// Return empty clusters config if file doesn't exist
		return clusters, nil
	}

	data, err := os.ReadFile(clustersPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read clusters file: %w", err)
	}

	if err := yaml.Unmarshal(data, clusters); err != nil {
		return nil, fmt.Errorf("failed to parse clusters file: %w", err)
	}

	return clusters, nil
}

func SaveClusters(clusters *models.ClustersConfig) error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	clustersPath := filepath.Join(configDir, ClustersFile)
	data, err := yaml.Marshal(clusters)
	if err != nil {
		return fmt.Errorf("failed to marshal clusters: %w", err)
	}

	return os.WriteFile(clustersPath, data, 0644)
}

// MergeWithFlags merges configuration with command line flags and environment variables
// Priority: flags > config file > environment variables
func MergeWithFlags(config *models.Config, spotifyID, spotifySecret, slskUsername, slskPassword, workingDir string) {
	if spotifyID != "" {
		config.SpotifyID = spotifyID
	} else if envID := os.Getenv("SPOTIFY_ID"); envID != "" && config.SpotifyID == "" {
		config.SpotifyID = envID
	}

	if spotifySecret != "" {
		config.SpotifySecret = spotifySecret
	} else if envSecret := os.Getenv("SPOTIFY_SECRET"); envSecret != "" && config.SpotifySecret == "" {
		config.SpotifySecret = envSecret
	}

	if slskUsername != "" {
		config.SlskUsername = slskUsername
	} else if envUsername := os.Getenv("SLSK_USERNAME"); envUsername != "" && config.SlskUsername == "" {
		config.SlskUsername = envUsername
	}

	if slskPassword != "" {
		config.SlskPassword = slskPassword
	} else if envPassword := os.Getenv("SLSK_PASSWORD"); envPassword != "" && config.SlskPassword == "" {
		config.SlskPassword = envPassword
	}

	if workingDir != "" {
		config.WorkingDir = workingDir
	} else if envDir := os.Getenv("WORKING_DIR"); envDir != "" && config.WorkingDir == DefaultWorkingDir {
		config.WorkingDir = envDir
	}
}

func ValidateConfig(config *models.Config) error {
	if config.SpotifyID == "" {
		return fmt.Errorf("spotify ID is required (--spotify-id, config file, or SPOTIFY_ID env var)")
	}
	if config.SpotifySecret == "" {
		return fmt.Errorf("spotify secret is required (--spotify-secret, config file, or SPOTIFY_SECRET env var)")
	}

	// SLSK credentials are optional but warn if missing
	if config.SlskUsername == "" {
		fmt.Printf("Warning: SLSK username not provided (--slsk-username, config file, or SLSK_USERNAME env var). Downloads may fail.\n")
	}
	if config.SlskPassword == "" {
		fmt.Printf("Warning: SLSK password not provided (--slsk-password, config file, or SLSK_PASSWORD env var). Downloads may fail.\n")
	}

	return nil
}
