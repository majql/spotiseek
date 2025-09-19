package spotify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"spotiseek/pkg/models"
)

const (
	AuthURL = "https://accounts.spotify.com/api/token"
	BaseURL = "https://api.spotify.com/v1"
)

type Client struct {
	clientID     string
	clientSecret string
	accessToken  string
	expiresAt    time.Time
	httpClient   *http.Client
}

func NewClient(clientID, clientSecret string) *Client {
	return &Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) authenticate() error {
	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequest("POST", AuthURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create auth request: %w", err)
	}

	req.SetBasicAuth(c.clientID, c.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make auth request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth request failed with status %d", resp.StatusCode)
	}

	var authResp models.SpotifyAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("failed to decode auth response: %w", err)
	}

	c.accessToken = authResp.AccessToken
	c.expiresAt = time.Now().Add(time.Duration(authResp.ExpiresIn) * time.Second)

	return nil
}

func (c *Client) ensureAuth() error {
	if c.accessToken == "" || time.Now().After(c.expiresAt) {
		return c.authenticate()
	}
	return nil
}

func (c *Client) makeRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	if err := c.ensureAuth(); err != nil {
		return nil, err
	}

	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	req, err := http.NewRequest(method, BaseURL+endpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	return resp, nil
}

// ExtractPlaylistID extracts playlist ID from Spotify URL or returns the ID if already in ID format
func ExtractPlaylistID(input string) (string, error) {
	// If it's already a playlist ID (alphanumeric string), return it
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9]+$`, input); matched {
		return input, nil
	}

	// Try to extract from Spotify URL
	re := regexp.MustCompile(`playlist/([a-zA-Z0-9]+)`)
	matches := re.FindStringSubmatch(input)
	if len(matches) > 1 {
		return matches[1], nil
	}

	return "", fmt.Errorf("invalid playlist ID or URL: %s", input)
}

func (c *Client) GetPlaylist(playlistID string) (*models.Playlist, error) {
	resp, err := c.makeRequest("GET", fmt.Sprintf("/playlists/%s", playlistID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode playlist response: %w", err)
	}

	return &models.Playlist{
		ID:   result.ID,
		Name: result.Name,
	}, nil
}

func (c *Client) GetPlaylistTracks(playlistID string) ([]models.Track, error) {
	var allTracks []models.Track
	offset := 0
	limit := 50

	for {
		endpoint := fmt.Sprintf("/playlists/%s/tracks?offset=%d&limit=%d&fields=items(added_at,track(id,name,artists(id,name),duration_ms))", playlistID, offset, limit)
		resp, err := c.makeRequest("GET", endpoint, nil)
		if err != nil {
			return nil, err
		}

		var result struct {
			Items []struct {
				AddedAt time.Time `json:"added_at"`
				Track   struct {
					ID       string          `json:"id"`
					Name     string          `json:"name"`
					Artists  []models.Artist `json:"artists"`
					Duration int             `json:"duration_ms"`
				} `json:"track"`
			} `json:"items"`
			Total int `json:"total"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode tracks response: %w", err)
		}
		resp.Body.Close()

		for _, item := range result.Items {
			if item.Track.ID != "" { // Skip null tracks
				track := models.Track{
					ID:       item.Track.ID,
					Name:     item.Track.Name,
					Artists:  item.Track.Artists,
					AddedAt:  item.AddedAt,
					Duration: item.Track.Duration,
				}
				allTracks = append(allTracks, track)
			}
		}

		offset += len(result.Items)
		if offset >= result.Total {
			break
		}
	}

	return allTracks, nil
}

func (c *Client) GetNewTracks(playlistID string, since time.Time) ([]models.Track, error) {
	tracks, err := c.GetPlaylistTracks(playlistID)
	if err != nil {
		return nil, err
	}

	log.Printf("Retrieved %d total tracks from playlist %s", len(tracks), playlistID)
	log.Printf("Looking for tracks added after: %v", since)

	var newTracks []models.Track
	for i, track := range tracks {
		isNew := track.AddedAt.After(since)
		// Log last 5 tracks or any new tracks for debugging (new tracks are usually at the end)
		if i >= len(tracks)-5 || isNew {
			log.Printf("Track %d: '%s' by %s - Added: %v (New: %v)",
				i+1, track.Name, c.formatArtists(track.Artists), track.AddedAt, isNew)
		}

		if isNew {
			newTracks = append(newTracks, track)
		}
	}

	log.Printf("Found %d new tracks out of %d total", len(newTracks), len(tracks))
	return newTracks, nil
}

func (c *Client) formatArtists(artists []models.Artist) string {
	if len(artists) == 0 {
		return "Unknown Artist"
	}
	if len(artists) == 1 {
		return artists[0].Name
	}
	names := make([]string, len(artists))
	for i, artist := range artists {
		names[i] = artist.Name
	}
	return strings.Join(names, ", ")
}
