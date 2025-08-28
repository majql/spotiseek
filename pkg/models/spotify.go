package models

import "time"

type Track struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Artists  []Artist  `json:"artists"`
	AddedAt  time.Time `json:"added_at"`
	Duration int       `json:"duration_ms"`
}

type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Playlist struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Tracks []Track `json:"tracks"`
}

type SpotifyAuthResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}