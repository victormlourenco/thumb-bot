package fxtwitter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
)

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Tweet   Tweet  `json:"tweet"`
}

type Tweet struct {
	URL               string  `json:"url"`
	ID                string  `json:"id"`
	Text              string  `json:"text"`
	RawText           RawText `json:"raw_text"`
	Author            Author  `json:"author"`
	Replies           int     `json:"replies"`
	Retweets          int     `json:"retweets"`
	Likes             int     `json:"likes"`
	Bookmarks         int     `json:"bookmarks"`
	CreatedAt         string  `json:"created_at"`
	CreatedTimestamp  int64   `json:"created_timestamp"`
	PossiblySensitive bool    `json:"possibly_sensitive"`
	Views             int     `json:"views"`
	IsNoteTweet       bool    `json:"is_note_tweet"`
	CommunityNote     *string `json:"community_note"`
	Lang              string  `json:"lang"`
	ReplyingTo        *string `json:"replying_to"`
	ReplyingToStatus  *string `json:"replying_to_status"`
	Media             *Media  `json:"media"`
	Source            string  `json:"source"`
	TwitterCard       string  `json:"twitter_card"`
	Color             *string `json:"color"`
	Provider          string  `json:"provider"`
}

type RawText struct {
	Text   string  `json:"text"`
	Facets []Facet `json:"facets"`
}

type Facet struct {
	Type        string `json:"type"`
	Indices     []int  `json:"indices"`
	ID          string `json:"id"`
	Display     string `json:"display"`
	Original    string `json:"original"`
	Replacement string `json:"replacement"`
}

type Author struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	ScreenName  string   `json:"screen_name"`
	AvatarURL   string   `json:"avatar_url"`
	BannerURL   string   `json:"banner_url"`
	Description string   `json:"description"`
	Location    string   `json:"location"`
	URL         string   `json:"url"`
	Followers   int      `json:"followers"`
	Following   int      `json:"following"`
	Joined      string   `json:"joined"`
	Likes       int      `json:"likes"`
	MediaCount  int      `json:"media_count"`
	Protected   bool     `json:"protected"`
	Website     *Website `json:"website"`
	Tweets      int      `json:"tweets"`
	AvatarColor *string  `json:"avatar_color"`
}

type Website struct {
	URL        string `json:"url"`
	DisplayURL string `json:"display_url"`
}

type Media struct {
	All    []MediaItem `json:"all"`
	Videos []MediaItem `json:"videos"`
}

type MediaItem struct {
	URL          string    `json:"url"`
	ThumbnailURL string    `json:"thumbnail_url"`
	Duration     float64   `json:"duration"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	Format       string    `json:"format"`
	Type         string    `json:"type"`
	Variants     []Variant `json:"variants"`
}

type Variant struct {
	ContentType string `json:"content_type"`
	URL         string `json:"url"`
	Bitrate     *int   `json:"bitrate"`
}

func Fetch(status string) (Response, error) {
	url := fmt.Sprintf("https://api.fxtwitter.com%s", status)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Response{}, fmt.Errorf("failed to create request: %w", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("failed to fetch tweet: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return Response{}, fmt.Errorf("failed to read response body: %w", err)
	}

	response := Response{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return Response{}, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	if response.Code != 200 {
		return Response{}, fmt.Errorf("API error: %s", response.Message)
	}

	return response, nil
}

// EstimateFileSize estimates file size in bytes using bitrate and duration
func EstimateFileSize(bitrate int, duration float64) int64 {
	// Convert bitrate from bps to bytes per second
	bytesPerSecond := int64(bitrate) / 8
	// Calculate total size
	return int64(duration * float64(bytesPerSecond))
}

// SelectBestVariant selects the best video variant under the size limit
func SelectBestVariant(variants []Variant, duration float64, maxSizeBytes int64) (*Variant, bool) {
	if len(variants) == 0 {
		return nil, false
	}

	// Filter variants with bitrate information and estimate their sizes
	var validVariants []struct {
		variant *Variant
		size    int64
	}

	for i := range variants {
		if variants[i].Bitrate != nil {
			size := EstimateFileSize(*variants[i].Bitrate, duration)
			validVariants = append(validVariants, struct {
				variant *Variant
				size    int64
			}{&variants[i], size})
		}
	}

	if len(validVariants) == 0 {
		return nil, false
	}

	// Sort by size (ascending) to get the largest variant under the limit
	sort.Slice(validVariants, func(i, j int) bool {
		return validVariants[i].size < validVariants[j].size
	})

	// Find the largest variant under the size limit
	for i := len(validVariants) - 1; i >= 0; i-- {
		if validVariants[i].size <= maxSizeBytes {
			return validVariants[i].variant, true
		}
	}

	// No variant fits under the limit
	return nil, false
}

// GetBestMediaForTelegram selects the best media item for Telegram based on size constraints
func GetBestMediaForTelegram(media MediaItem) (string, string, bool) {
	const maxSizeBytes = 20 * 1024 * 1024 // 20MB

	// For videos, try to find the best variant
	if media.Type == "video" && len(media.Variants) > 0 {
		bestVariant, found := SelectBestVariant(media.Variants, media.Duration, maxSizeBytes)
		if found {
			return bestVariant.URL, "video", true
		}
		// If no variant fits, fall back to thumbnail
		if media.ThumbnailURL != "" {
			return media.ThumbnailURL, "photo", true
		}
	}

	// For photos or if no variants available, use the original URL
	if media.Type == "photo" || media.Type == "image" {
		return media.URL, "photo", true
	}

	// For videos without variants, use original URL (might exceed size limit)
	if media.Type == "video" {
		return media.URL, "video", true
	}

	return "", "", false
}
