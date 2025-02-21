package vxtwitter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Response struct {
	ConversationID string   `json:"conversationID"`
	Date           string   `json:"date"`
	DateEpoch      int      `json:"date_epoch"`
	Hashtags       []any    `json:"hashtags"`
	Likes          int      `json:"likes"`
	MediaURLs      []string `json:"mediaURLs"`
	MediaExtended  []struct {
		AltText        any `json:"altText"`
		DurationMillis int `json:"duration_millis"`
		Size           struct {
			Height int `json:"height"`
			Width  int `json:"width"`
		} `json:"size"`
		ThumbnailURL string `json:"thumbnail_url"`
		Type         string `json:"type"`
		URL          string `json:"url"`
	} `json:"media_extended"`
	PossiblySensitive bool   `json:"possibly_sensitive"`
	QrtURL            any    `json:"qrtURL"`
	Replies           int    `json:"replies"`
	Retweets          int    `json:"retweets"`
	Text              string `json:"text"`
	TweetID           string `json:"tweetID"`
	TweetURL          string `json:"tweetURL"`
	UserName          string `json:"user_name"`
	UserScreenName    string `json:"user_screen_name"`
}

func Fetch(status string) (Response, error) {
	url := fmt.Sprintf("https://api.vxtwitter.com%s", status)
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

	return response, nil
}
