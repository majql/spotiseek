package utils

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
	"spotiseek/pkg/models"
)

// TransliterateToASCII converts Unicode characters to ASCII equivalents
func TransliterateToASCII(input string) string {
	// First handle specific character mappings
	charMappings := map[rune]string{
		'ø': "o", 'Ø': "O",
		'æ': "ae", 'Æ': "AE",
		'å': "a", 'Å': "A",
		'ü': "u", 'Ü': "U",
		'ö': "o", 'Ö': "O",
		'ä': "a", 'Ä': "A",
		'ñ': "n", 'Ñ': "N",
		'ç': "c", 'Ç': "C",
		'é': "e", 'É': "E",
		'è': "e", 'È': "E",
		'ê': "e", 'Ê': "E",
		'ë': "e", 'Ë': "E",
		'á': "a", 'Á': "A",
		'à': "a", 'À': "A",
		'â': "a", 'Â': "A",
		'ã': "a", 'Ã': "A",
		'í': "i", 'Í': "I",
		'ì': "i", 'Ì': "I",
		'î': "i", 'Î': "I",
		'ï': "i", 'Ï': "I",
		'ó': "o", 'Ó': "O",
		'ò': "o", 'Ò': "O",
		'ô': "o", 'Ô': "O",
		'õ': "o", 'Õ': "O",
		'ú': "u", 'Ú': "U",
		'ù': "u", 'Ù': "U",
		'û': "u", 'Û': "U",
		'ÿ': "y", 'Ÿ': "Y",
		'ý': "y", 'Ý': "Y",
	}
	
	var result strings.Builder
	for _, r := range input {
		if mapped, exists := charMappings[r]; exists {
			result.WriteString(mapped)
		} else if r <= 127 { // ASCII character
			result.WriteRune(r)
		} else {
			// For other Unicode characters, try NFD normalization
			normalized := norm.NFD.String(string(r))
			// Remove combining marks and take the base character
			for _, nr := range normalized {
				if !unicode.Is(unicode.Mn, nr) && nr <= 127 {
					result.WriteRune(nr)
					break
				}
			}
		}
	}
	
	return result.String()
}

// NormalizeString converts string to normalized form for matching
func NormalizeString(input string) string {
	// First transliterate to ASCII
	result := TransliterateToASCII(input)

	// Convert to lowercase
	result = strings.ToLower(result)

	// Remove non-alphanumeric characters except spaces
	reg := regexp.MustCompile(`[^a-z0-9\s]`)
	result = reg.ReplaceAllString(result, " ")

	// Replace multiple spaces with single space and trim
	spaceReg := regexp.MustCompile(`\s+`)
	result = spaceReg.ReplaceAllString(result, " ")
	result = strings.TrimSpace(result)

	return result
}

// CreateSearchQuery creates a search query from track information
func CreateSearchQuery(track models.Track) string {
	var parts []string

	// Add all artist names
	for _, artist := range track.Artists {
		parts = append(parts, artist.Name)
	}

	// Add track name
	parts = append(parts, track.Name)

	// Join and normalize
	query := strings.Join(parts, " ")
	return NormalizeString(query)
}

// MatchScore represents a match score with details
type MatchScore struct {
	Score    float64
	Filename string
	Reason   string
}

// extractRelevantFilename extracts the most relevant part of the filename for matching
func extractRelevantFilename(filename string) string {
	// Get the actual filename (after last slash)
	parts := strings.Split(filename, "/")
	actualFilename := parts[len(parts)-1]
	
	// Also include the parent directory for context if available
	if len(parts) >= 2 {
		parentDir := parts[len(parts)-2]
		actualFilename = parentDir + " " + actualFilename
	}
	
	return actualFilename
}

// CalculateMatchScore calculates how well a filename matches the query
func CalculateMatchScore(query, filename string) MatchScore {
	normalizedQuery := NormalizeString(query)
	// Focus on the relevant part of the filename
	relevantFilename := extractRelevantFilename(filename)
	normalizedFilename := NormalizeString(relevantFilename)

	queryWords := strings.Fields(normalizedQuery)
	filenameWords := strings.Fields(normalizedFilename)

	if len(queryWords) == 0 {
		return MatchScore{Score: 0, Filename: filename, Reason: "empty query"}
	}

	// Check for exact match
	if normalizedQuery == normalizedFilename {
		return MatchScore{Score: 1.0, Filename: filename, Reason: "exact match"}
	}

	// Count matching words
	queryWordSet := make(map[string]bool)
	for _, word := range queryWords {
		queryWordSet[word] = true
	}

	matchingWords := 0
	for _, word := range filenameWords {
		if queryWordSet[word] {
			matchingWords++
		}
	}

	// Base score is ratio of matching words to query words
	baseScore := float64(matchingWords) / float64(len(queryWords))

	// Bonus points for exact word sequence matches
	sequenceBonus := 0.0
	if matchingWords > 1 {
		queryText := strings.Join(queryWords, " ")
		if strings.Contains(normalizedFilename, queryText) {
			sequenceBonus = 0.2
		}
	}

	// Bonus for "original" versions
	originalBonus := 0.0
	originalTerms := []string{"original", "original mix", "original version"}
	for _, term := range originalTerms {
		if strings.Contains(normalizedFilename, term) {
			originalBonus = 0.1
			break
		}
	}

	// Reduced penalty for extra words (less harsh for files with metadata)
	extraWordsPenalty := 0.0
	extraWords := len(filenameWords) - matchingWords
	if extraWords > 3 {
		extraWordsPenalty = float64(extraWords-3) * 0.02
	}

	finalScore := baseScore + sequenceBonus + originalBonus - extraWordsPenalty
	if finalScore > 1.0 {
		finalScore = 1.0
	}
	if finalScore < 0 {
		finalScore = 0
	}

	reason := fmt.Sprintf("base:%.2f seq:%.2f orig:%.2f penalty:%.2f (%d/%d words)", 
		baseScore, sequenceBonus, originalBonus, extraWordsPenalty, matchingWords, len(queryWords))

	return MatchScore{
		Score:    finalScore,
		Filename: filename,
		Reason:   reason,
	}
}


// FilterMP3Files filters search results to only include MP3 files
func FilterMP3Files(results []models.SearchResult) []models.SearchResult {
	var mp3Files []models.SearchResult
	for _, result := range results {
		ext := strings.ToLower(filepath.Ext(result.Filename))
		if ext == ".mp3" {
			mp3Files = append(mp3Files, result)
		}
	}
	return mp3Files
}

// FindBestMatch finds the best matching file from search results
func FindBestMatch(query string, results []models.SearchResult) *models.SearchResult {
	// First filter to only MP3 files
	mp3Results := FilterMP3Files(results)
	if len(mp3Results) == 0 {
		log.Printf("No MP3 files found in %d search results", len(results))
		return nil
	}

	log.Printf("Filtering %d results to %d MP3 files", len(results), len(mp3Results))

	// Calculate scores for all MP3 files
	var scores []MatchScore
	for _, result := range mp3Results {
		score := CalculateMatchScore(query, result.Filename)
		scores = append(scores, score)
	}

	// Sort by score descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	// Log top matches for analysis
	log.Printf("Match analysis for query: %s", query)
	maxToLog := 5
	if len(scores) < maxToLog {
		maxToLog = len(scores)
	}
	
	for i := 0; i < maxToLog; i++ {
		log.Printf("  %d. Score: %.3f - %s (%s)", 
			i+1, scores[i].Score, scores[i].Filename, scores[i].Reason)
	}

	// Return best match if score is good enough (lowered threshold)
	if len(scores) > 0 && scores[0].Score >= 0.15 {
		// Find the result object for the best match
		for _, result := range mp3Results {
			if result.Filename == scores[0].Filename {
				log.Printf("Selected best match: %s (score: %.3f)", result.Filename, scores[0].Score)
				return &result
			}
		}
	}

	log.Printf("No suitable match found (best score: %.3f)", scores[0].Score)
	return nil
}

// LogMatchDecision logs detailed information about the match decision
func LogMatchDecision(query string, results []models.SearchResult, selected *models.SearchResult) {
	log.Printf("=== MATCH DECISION LOG ===")
	log.Printf("Query: %s", query)
	log.Printf("Normalized query: %s", NormalizeString(query))
	log.Printf("Total results: %d", len(results))
	
	mp3Results := FilterMP3Files(results)
	log.Printf("MP3 results: %d", len(mp3Results))
	
	if selected != nil {
		log.Printf("SELECTED: %s", selected.Filename)
		score := CalculateMatchScore(query, selected.Filename)
		log.Printf("  Score: %.3f (%s)", score.Score, score.Reason)
		log.Printf("  User: %s", selected.Username)
		log.Printf("  Size: %d bytes", selected.Size)
	} else {
		log.Printf("SELECTED: None (no suitable match)")
	}
	
	log.Printf("========================")
}