# Spotiseek Implementation Plan

## Project Overview
Spotiseek is a Go-based application that monitors Spotify playlists for new tracks and automatically downloads them via Slskd (Soulseek P2P client) using Docker containers.

## Architecture Components

### 1. Main Application (`spotiseek`)
- **Purpose**: Main executable that manages Docker clusters
- **Responsibilities**:
  - Parse CLI arguments and configuration
  - Manage Docker clusters (create/destroy)
  - Store cluster metadata
  - Handle playlist watching/forgetting operations

### 2. Worker Application (`worker`)
- **Purpose**: Container-based worker that monitors a single playlist
- **Responsibilities**:
  - Monitor Spotify playlist for new tracks
  - Search for tracks in Slskd
  - Download best matching files
  - Handle retry logic for Slskd connectivity

## Implementation Phases

### Phase 1: Project Structure & Dependencies
```
spotiseek/
├── cmd/
│   ├── spotiseek/
│   │   └── main.go
│   └── worker/
│       └── main.go
├── internal/
│   ├── config/
│   ├── docker/
│   ├── spotify/
│   ├── slskd/
│   └── utils/
├── pkg/
│   └── models/
├── go.mod
├── go.sum
├── Dockerfile.worker
└── docker-compose.template.yml
```

**Tasks:**
- Initialize Go module
- Set up directory structure
- Install required dependencies:
  - Docker Go SDK (`github.com/docker/docker`)
  - Spotify Web API client
  - HTTP client for Slskd API
  - YAML parsing library
  - String transliteration library

### Phase 2: Configuration Management

**Configuration Files:**
- `~/.spotiseek/spotiseek.yml` - Main configuration
- `~/.spotiseek/clusters.yml` - Active cluster tracking

**Configuration Structure:**
```yaml
# spotiseek.yml
spotify_id: "your_spotify_id"
spotify_secret: "your_spotify_secret" 
working_dir: "~/spotiseek"

# clusters.yml
clusters:
  - playlist_id: "playlist123"
    container_names:
      worker: "spotiseek-playlist123-worker"
      slskd: "spotiseek-playlist123-slskd"
    network_name: "spotiseek-playlist123"
    created_at: "2024-01-01T00:00:00Z"
```

**Tasks:**
- Implement config parsing with precedence (CLI > config > env)
- Create config validation
- Handle config file creation and updates

### Phase 3: Spotify Integration

**API Integration:**
- Authentication using client credentials flow
- Playlist metadata retrieval
- Track change detection

**Key Functions:**
```go
type SpotifyClient struct {
    clientID     string
    clientSecret string
    accessToken  string
}

func (s *SpotifyClient) GetPlaylistTracks(playlistID string) ([]Track, error)
func (s *SpotifyClient) HasNewTracks(playlistID string, lastCheck time.Time) ([]Track, error)
```

**Tasks:**
- Implement Spotify Web API client
- Handle authentication and token refresh
- Create track comparison logic

### Phase 4: Docker Management

**Docker Operations:**
- Network creation/deletion
- Container creation/management
- Volume mounting for downloads
- Container health monitoring

**Key Functions:**
```go
type DockerManager struct {
    client *client.Client
}

func (d *DockerManager) CreateCluster(playlistID string, config ClusterConfig) error
func (d *DockerManager) DestroyCluster(playlistID string) error
func (d *DockerManager) GetClusterStatus(playlistID string) (ClusterStatus, error)
```

**Docker Resources:**
- **Network**: `spotiseek-{PLAYLIST_ID}`
- **Containers**:
  - Worker: `spotiseek-{PLAYLIST_ID}-worker`
  - Slskd: `spotiseek-{PLAYLIST_ID}-slskd`
- **Volumes**: Host directory mounted to both containers

**Tasks:**
- Implement Docker client wrapper
- Create cluster lifecycle management
- Handle Docker image pulling and container orchestration

### Phase 5: Slskd Integration

**API Integration:**
- Search initiation and monitoring
- File download management
- Connection retry logic

**Key Functions:**
```go
type SlskdClient struct {
    baseURL string
    client  *http.Client
}

func (s *SlskdClient) Search(query string) (string, error) // returns search ID
func (s *SlskdClient) GetSearchStatus(searchID string) (SearchStatus, error)
func (s *SlskdClient) GetSearchResults(searchID string) ([]SearchResult, error)
func (s *SlskdClient) DownloadFile(username, filename string) error
```

**Tasks:**
- Implement Slskd REST API client
- Create search monitoring with polling
- Handle connection failures with exponential backoff

### Phase 6: String Processing & Matching

**Text Processing:**
- Transliteration to ASCII
- Normalization (remove special characters)
- String similarity matching

**Matching Algorithm:**
```go
func NormalizeString(input string) string
func TransliterateToASCII(input string) string
func FindBestMatch(query string, results []SearchResult) SearchResult
func CalculateMatchScore(query, candidate string) float64
```

**Matching Criteria:**
1. Exact word matches (highest priority)
2. Partial word matches with tolerance
3. "Original mix/version" preference
4. File extension filtering (MP3 only)

**Tasks:**
- Implement transliteration using Unicode normalization
- Create string matching algorithm with scoring
- Add comprehensive logging for match decisions

### Phase 7: Worker Implementation

**Core Worker Logic:**
```go
type Worker struct {
    spotifyClient *SpotifyClient
    slskdClient   *SlskdClient
    config        WorkerConfig
    lastCheck     time.Time
}

func (w *Worker) Run() error
func (w *Worker) checkForNewTracks() ([]Track, error)
func (w *Worker) processTrack(track Track) error
func (w *Worker) searchAndDownload(track Track) error
```

**Concurrency:**
- Use goroutines for parallel track processing
- Implement worker pools for search/download operations
- Handle graceful shutdown

**Tasks:**
- Implement main worker loop with interval-based checking
- Create concurrent track processing
- Add comprehensive error handling and logging

### Phase 8: Main Application CLI

**CLI Interface:**
```bash
spotiseek watch <playlist_id_or_url> [flags]
spotiseek forget <playlist_id_or_url> [flags]
```

**Operations:**
- Parse playlist ID from URL or use directly
- Validate configuration
- Create/destroy Docker clusters
- Update cluster registry

**Tasks:**
- Implement CLI parsing with cobra or similar
- Create cluster management commands
- Add validation and error handling

### Phase 9: Error Handling & Logging

**Error Scenarios:**
- Spotify API failures
- Docker daemon unavailable
- Slskd connection issues
- File system permissions
- Invalid configuration

**Logging Strategy:**
- Structured logging with levels (debug, info, warn, error)
- Separate log files for main app and workers
- Match decision logging for analysis

**Tasks:**
- Implement comprehensive error handling
- Add structured logging throughout
- Create health check mechanisms

### Phase 10: Testing & Documentation

**Testing Strategy:**
- Unit tests for core algorithms
- Integration tests with Docker
- Mock Spotify and Slskd APIs for testing
- End-to-end testing with real playlists

**Documentation:**
- API documentation
- Configuration examples
- Troubleshooting guide
- Development setup instructions

**Tasks:**
- Write comprehensive test suite
- Create user documentation
- Add code documentation and examples

## Technical Decisions

### Concurrency Model
- Use goroutines with worker pools for I/O operations
- Channel-based communication for coordination
- Context-based cancellation for cleanup

### Configuration Precedence
1. Command-line flags (highest)
2. Configuration file
3. Environment variables (lowest)

### Error Recovery
- Exponential backoff for API failures
- Container restart policies
- Graceful degradation when possible

### Security Considerations
- Store API credentials securely
- Validate all external inputs
- Use least-privilege Docker containers
- Audit file download locations

## Deployment Considerations

### Docker Images
- Multi-stage builds for minimal image size
- Security scanning for vulnerabilities
- Version tagging strategy

### Monitoring
- Health checks for containers
- Metrics collection for search success rates
- Log aggregation for debugging

### Resource Management
- Memory limits for containers
- Disk space monitoring for downloads
- Network bandwidth considerations

## Estimated Timeline

- **Phase 1-2**: 2-3 days (Setup & Config)
- **Phase 3**: 2-3 days (Spotify Integration)
- **Phase 4**: 3-4 days (Docker Management)
- **Phase 5**: 2-3 days (Slskd Integration)
- **Phase 6**: 2-3 days (String Processing)
- **Phase 7**: 3-4 days (Worker Implementation)
- **Phase 8**: 1-2 days (CLI Interface)
- **Phase 9**: 2-3 days (Error Handling)
- **Phase 10**: 3-4 days (Testing & Docs)

**Total Estimated Time**: 3-4 weeks

## Success Criteria

1. Successfully monitors Spotify playlists for new tracks
2. Automatically searches and downloads matching files via Slskd
3. Handles multiple playlists concurrently using Docker clusters
4. Provides reliable string matching with configurable parameters
5. Includes comprehensive error handling and logging
6. Maintains cluster state persistently across restarts