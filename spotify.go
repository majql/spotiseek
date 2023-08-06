package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/zmb3/spotify"
	"golang.org/x/oauth2/clientcredentials"
)

func main() {
	config := &clientcredentials.Config{
		ClientID:     os.Getenv("SPOTIFY_ID"),
		ClientSecret: os.Getenv("SPOTIFY_SECRET"),
		TokenURL:     spotify.TokenURL,
	}
	token, err := config.Token(context.Background())
	if err != nil {
		log.Fatalf("couldn't get token: %v", err)
	}

	client := spotify.Authenticator{}.NewClient(token)

	tracks, err := client.GetPlaylistTracks("0ICI46XxAvf56sus9c3XbQ")
	if err != nil {
		log.Fatal(err)
	}

	for _, artist := range tracks.Tracks[0].Track.Artists {
		fmt.Println(artist.Name)
	}

	log.Printf("Playlist has %d total tracks", tracks.Total)
	for page := 1; ; page++ {
		log.Printf("  Page %d has %d tracks", page, len(tracks.Tracks))
		err = client.NextPage(tracks)
		if err == spotify.ErrNoMorePages {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
	}
}
