package ApiClients

import (
	"context"
	"fmt"
	spotifyVendored "github.com/zmb3/spotify"
	"golang.org/x/oauth2/clientcredentials"
	"log"
	"strings"
	"time"
)

type SpotifyService struct {
	client spotifyVendored.Client
}

type Spotify interface {
	SpotifyService

	Auth() bool
	Search() string
}

func NewSpotify(clientId string, clientSecret string) *SpotifyService {
	config := &clientcredentials.Config{
		ClientID:     clientId,
		ClientSecret: clientSecret,
		TokenURL:     spotifyVendored.TokenURL,
	}
	token, err := config.Token(context.Background())
	if err != nil {
		log.Fatalf("couldn't get token: %v", err)
	}

	realClient := spotifyVendored.Authenticator{}.NewClient(token)
	return &SpotifyService{
		client: realClient,
	}

}

func (spotifyService *SpotifyService) Auth() bool {
	return true
}

func (spotifyService *SpotifyService) GetPlaylistTracks(playlistId string, after time.Time) []string {
	tracks, err := spotifyService.client.GetPlaylistTracks(spotifyVendored.ID(playlistId))
	if err != nil {
		log.Fatal(err)
	}

	var playlistContents []string
	for _, track := range tracks.Tracks {
		trackTime, _ := time.Parse(time.RFC3339, track.AddedAt)
		if !trackTime.After(after) {
			//fmt.Println(track.Track.Name, trackTime.GoString(), after.GoString(), "Continuing")
			continue
		}

		var artistsFull []string

		for _, artists := range track.Track.Artists {
			artistsFull = append(artistsFull, artists.Name)
		}

		entryFull := fmt.Sprintf("%s %s", strings.Join(artistsFull, " "), track.Track.Name)
		log.Printf("Found playlist entry: '%s'", entryFull)
		playlistContents = append(playlistContents, entryFull)
	}

	return playlistContents
}

//func (spotifyService *SpotifyService) Search(query string) string {
//	return "ad"
//}

//type Spotify interface {
//	auth() bool
//}
