package youtube

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// ExtractVideoID extracts the video ID from various YouTube URL formats
func ExtractVideoID(youtubeURL string) (string, error) {
	parsedURL, err := url.Parse(youtubeURL)
	if err != nil {
		return "", err
	}

	// Handle different YouTube URL formats
	// https://www.youtube.com/watch?v=VIDEO_ID
	// https://youtube.com/watch?v=VIDEO_ID
	// https://youtu.be/VIDEO_ID
	// https://www.youtube.com/embed/VIDEO_ID
	// https://m.youtube.com/watch?v=VIDEO_ID

	// Check for youtu.be short links
	if parsedURL.Host == "youtu.be" || parsedURL.Host == "www.youtu.be" {
		videoID := strings.TrimPrefix(parsedURL.Path, "/")
		// Remove any trailing path segments or query params from the video ID
		videoID = strings.Split(videoID, "/")[0]
		videoID = strings.Split(videoID, "?")[0]
		if videoID != "" {
			return videoID, nil
		}
	}

	// Check for standard watch URLs
	if strings.Contains(parsedURL.Path, "/watch") {
		videoID := parsedURL.Query().Get("v")
		if videoID != "" {
			return videoID, nil
		}
	}

	// Check for embed URLs
	if strings.Contains(parsedURL.Path, "/embed/") {
		parts := strings.Split(parsedURL.Path, "/embed/")
		if len(parts) > 1 {
			videoID := strings.Split(parts[1], "?")[0]
			if videoID != "" {
				return videoID, nil
			}
		}
	}

	// Try regex as fallback
	re := regexp.MustCompile(`(?:youtube\.com\/watch\?v=|youtu\.be\/|youtube\.com\/embed\/)([a-zA-Z0-9_-]{11})`)
	matches := re.FindStringSubmatch(youtubeURL)
	if len(matches) > 1 {
		return matches[1], nil
	}

	return "", errors.New("could not extract video ID from URL")
}

// Fetch retrieves YouTube video information using oEmbed API
func Fetch(youtubeURL string) (YouTubeResponse, error) {
	videoID, err := ExtractVideoID(youtubeURL)
	if err != nil {
		return YouTubeResponse{}, fmt.Errorf("failed to extract video ID: %w", err)
	}

	// Normalize URL to standard format for oEmbed
	normalizedURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)

	// YouTube oEmbed API endpoint
	oEmbedURL := fmt.Sprintf("https://www.youtube.com/oembed?url=%s&format=json", url.QueryEscape(normalizedURL))

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(oEmbedURL)
	if err != nil {
		return YouTubeResponse{}, fmt.Errorf("failed to fetch YouTube data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return YouTubeResponse{}, fmt.Errorf("YouTube API returned status %d: %s", resp.StatusCode, string(body))
	}

	var response YouTubeResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return YouTubeResponse{}, fmt.Errorf("failed to decode YouTube response: %w", err)
	}

	return response, nil
}

// GetDirectLink returns a clean YouTube URL for the video
func GetDirectLink(youtubeURL string) (string, error) {
	videoID, err := ExtractVideoID(youtubeURL)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID), nil
}
