package app

import (
	"log"
	"testing"

	"spotiseek/internal/config"
	slsk "spotiseek/internal/soulseek"
)

func TestNormalizeText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Artist - Song Title", "artist song title"},
		{"Tëst Sóng (Remix)", "test song remix"},
		{"Track_with-dashes.mp3", "track with dashes mp3"},
		{"Łukasz Świątkowski", "lukasz swiatkowski"},
		{"123 Numbers & Symbols!", "123 numbers symbols"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeText(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeText(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"Artist - Song Title", []string{"artist", "song", "title"}},
		{"Tëst    Sóng", []string{"test", "song"}},
		{"", []string{}},
		{"   spaces   ", []string{"spaces"}},
		{"One", []string{"one"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := tokenize(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("tokenize(%q) = %v, want %v", tt.input, result, tt.expected)
				return
			}
			for i, token := range result {
				if token != tt.expected[i] {
					t.Errorf("tokenize(%q) = %v, want %v", tt.input, result, tt.expected)
					break
				}
			}
		})
	}
}

func TestCalculateMatchScore(t *testing.T) {
	app := &App{
		cfg:    config.Config{},
		logger: log.New(log.Writer(), "", 0),
	}

	tests := []struct {
		query    string
		filename string
		want     struct {
			exactMatch   bool
			partialMatch bool
			hasOriginal  bool
			tokenRatio   float64
		}
	}{
		{
			query:    "Artist Song",
			filename: "Artist - Song.mp3",
			want: struct {
				exactMatch   bool
				partialMatch bool
				hasOriginal  bool
				tokenRatio   float64
			}{true, false, false, 1.0},
		},
		{
			query:    "Artist Song",
			filename: "Artist - Song (Original Mix).mp3",
			want: struct {
				exactMatch   bool
				partialMatch bool
				hasOriginal  bool
				tokenRatio   float64
			}{true, false, true, 1.0},
		},
		{
			query:    "Artist Song",
			filename: "Artist - Different Song.mp3",
			want: struct {
				exactMatch   bool
				partialMatch bool
				hasOriginal  bool
				tokenRatio   float64
			}{true, false, false, 1.0}, // Both "artist" and "song" are present
		},
		{
			query:    "Artist Song Title",
			filename: "Artist - Song.mp3",
			want: struct {
				exactMatch   bool
				partialMatch bool
				hasOriginal  bool
				tokenRatio   float64
			}{false, true, false, 0.67}, // 2/3 = 0.67 (rounded)
		},
		{
			query:    "Artist Song",
			filename: "Artist - Song [Label123].mp3",
			want: struct {
				exactMatch   bool
				partialMatch bool
				hasOriginal  bool
				tokenRatio   float64
			}{true, false, false, 1.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.query+" vs "+tt.filename, func(t *testing.T) {
			score := app.calculateMatchScore(tt.query, tt.filename)
			
			if score.exactMatch != tt.want.exactMatch {
				t.Errorf("exactMatch = %t, want %t", score.exactMatch, tt.want.exactMatch)
			}
			if score.partialMatch != tt.want.partialMatch {
				t.Errorf("partialMatch = %t, want %t", score.partialMatch, tt.want.partialMatch)
			}
			if score.hasOriginal != tt.want.hasOriginal {
				t.Errorf("hasOriginal = %t, want %t", score.hasOriginal, tt.want.hasOriginal)
			}
			
			// Allow small floating point differences
			if abs(score.tokenRatio-tt.want.tokenRatio) > 0.01 {
				t.Errorf("tokenRatio = %.3f, want %.3f", score.tokenRatio, tt.want.tokenRatio)
			}
		})
	}
}

func TestSelectBest(t *testing.T) {
	app := &App{
		cfg:    config.Config{},
		logger: log.New(log.Writer(), "", 0),
	}

	responses := []slsk.Response{
		{
			Username:          "user1",
			QueueLength:       0,
			HasFreeUploadSlot: true,
			UploadSpeed:       1000,
			Files: []slsk.File{
				{Filename: "Artist - Song Title.mp3", Size: 5000000, IsLocked: false},
				{Filename: "Artist - Song Title (Remix).mp3", Size: 4500000, IsLocked: false},
				{Filename: "Artist - Song Title (Original Mix).mp3", Size: 5200000, IsLocked: false},
				{Filename: "Artist - Song Title.flac", Size: 25000000, IsLocked: false}, // Should be ignored
				{Filename: "Different - Artist Song.mp3", Size: 4000000, IsLocked: false},
			},
		},
		{
			Username:          "user2",
			QueueLength:       5,
			HasFreeUploadSlot: false,
			UploadSpeed:       500,
			Files: []slsk.File{
				{Filename: "Artist - Song Title (Perfect Match).mp3", Size: 6000000, IsLocked: false},
			},
		},
	}

	tests := []struct {
		name           string
		query          string
		expectUser     string
		expectFilename string
		expectError    bool
	}{
		{
			name:           "exact match prefers fewer extra tokens",
			query:          "Artist Song Title",
			expectUser:     "user1",
			expectFilename: "Artist - Song Title.mp3",
			expectError:    false,
		},
		{
			name:           "simple query prefers original",
			query:          "Artist Song",
			expectUser:     "user1",
			expectFilename: "Artist - Song Title (Original Mix).mp3",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			username, filename, _, err := app.selectBest(tt.query, responses)
			
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if username != tt.expectUser {
				t.Errorf("Expected user %s, got %s", tt.expectUser, username)
			}
			
			if filename != tt.expectFilename {
				t.Errorf("Expected filename %s, got %s", tt.expectFilename, filename)
			}
		})
	}
}

func TestSelectBestMP3Only(t *testing.T) {
	app := &App{
		cfg:    config.Config{},
		logger: log.New(log.Writer(), "", 0),
	}

	responses := []slsk.Response{
		{
			Username:          "user1",
			QueueLength:       0,
			HasFreeUploadSlot: true,
			UploadSpeed:       1000,
			Files: []slsk.File{
				{Filename: "Artist - Song.flac", Size: 25000000, IsLocked: false},
				{Filename: "Artist - Song.wav", Size: 50000000, IsLocked: false},
				{Filename: "Artist - Song.ogg", Size: 3000000, IsLocked: false},
				// No MP3 files
			},
		},
	}

	_, _, _, err := app.selectBest("Artist Song", responses)
	if err == nil {
		t.Error("Expected error when no MP3 files available")
	}
}

// Helper function for floating point comparison
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}