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

	// Fetch description from YouTube page (oEmbed doesn't provide it)
	description, err := fetchDescription(normalizedURL, client)
	if err == nil && description != "" {
		response.Description = description
	}

	return response, nil
}

// fetchDescription extracts the video description from YouTube page
func fetchDescription(youtubeURL string, client *http.Client) (string, error) {
	resp, err := client.Get(youtubeURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch YouTube page: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	bodyStr := string(body)

	// Try multiple methods to extract description

	// Method 1: Extract from meta tag (most reliable)
	// YouTube stores description in: <meta name="description" content="...">
	metaDescRegex := regexp.MustCompile(`<meta\s+name=["']description["']\s+content=["']([^"']+)["']`)
	matches := metaDescRegex.FindStringSubmatch(bodyStr)
	if len(matches) > 1 {
		desc := decodeHTMLEntities(matches[1])
		if desc != "" {
			return desc, nil
		}
	}

	// Method 2: Extract from JSON-LD structured data
	jsonLdRegex := regexp.MustCompile(`"description"\s*:\s*"((?:[^"\\]|\\.)*)"`)
	jsonMatches := jsonLdRegex.FindStringSubmatch(bodyStr)
	if len(jsonMatches) > 1 {
		desc := decodeHTMLEntities(jsonMatches[1])
		if desc != "" {
			return desc, nil
		}
	}

	// Method 3: Extract from ytInitialData (YouTube's initial data)
	ytDataRegex := regexp.MustCompile(`"shortDescription"\s*:\s*"((?:[^"\\]|\\.)*)"`)
	ytMatches := ytDataRegex.FindStringSubmatch(bodyStr)
	if len(ytMatches) > 1 {
		desc := decodeHTMLEntities(ytMatches[1])
		if desc != "" {
			return desc, nil
		}
	}

	return "", errors.New("description not found")
}

// decodeHTMLEntities decodes common HTML entities in the description
func decodeHTMLEntities(s string) string {
	// Replace common HTML entities
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&apos;", "'")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	// Handle unicode escapes like \u0026
	s = strings.ReplaceAll(s, "\\u0026", "&")
	s = strings.ReplaceAll(s, "\\u003c", "<")
	s = strings.ReplaceAll(s, "\\u003e", ">")
	return s
}

// GetDirectLink returns a clean YouTube URL for the video
func GetDirectLink(youtubeURL string) (string, error) {
	videoID, err := ExtractVideoID(youtubeURL)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID), nil
}
