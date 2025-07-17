package spotify

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zmb3/spotify"
	"golang.org/x/oauth2/clientcredentials"
)

type Client struct {
	api spotify.Client
}

func New(id, secret string) (*Client, error) {
	cfg := &clientcredentials.Config{
		ClientID:     id,
		ClientSecret: secret,
		TokenURL:     spotify.TokenURL,
	}
	token, err := cfg.Token(context.Background())
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	api := spotify.Authenticator{}.NewClient(token)
	return &Client{api: api}, nil
}

// PlaylistTracksAddedAfter returns track titles (artist – name) added after the provided timestamp.
func (c *Client) PlaylistTracksAddedAfter(playlistID string, after time.Time) ([]string, error) {
	pl, err := c.api.GetPlaylistTracks(spotify.ID(playlistID))
	if err != nil {
		return nil, err
	}

	var tracks []string
	for _, it := range pl.Tracks {
		addedAt, _ := time.Parse(time.RFC3339, it.AddedAt)
		if !addedAt.After(after) {
			continue
		}
		var artists []string
		for _, a := range it.Track.Artists {
			artists = append(artists, a.Name)
		}
		tracks = append(tracks, fmt.Sprintf("%s %s", strings.Join(artists, " "), it.Track.Name))
	}
	return tracks, nil
}
