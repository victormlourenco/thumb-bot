package bunker

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"thumb-bot/utils"
	"time"
)

const apiBase = "https://bnkr-integration.victormlourenco.workers.dev"

type Clip struct {
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	Status          string  `json:"status"`
	StartSeconds    float64 `json:"startSeconds"`
	DurationSeconds float64 `json:"durationSeconds"`
	Link            string  `json:"link"`
	Thumbnail       string  `json:"thumbnail"`
	Live            Live    `json:"live"`
	User            User    `json:"user"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
}

type Live struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Photo string `json:"photo"`
	Bio   string `json:"bio"`
}

func ExtractClipID(rawURL string) (string, error) {
	parsed, err := url.Parse(utils.RemoveQueryParams(rawURL))
	if err != nil {
		return "", err
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "clips" || parts[1] == "" {
		return "", errors.New("invalid bunker clip url")
	}

	return parts[1], nil
}

func Fetch(clipID, token string) (Clip, error) {
	if token == "" {
		return Clip{}, errors.New("bunker token is empty")
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/clip/%s", apiBase, clipID), nil)
	if err != nil {
		return Clip{}, fmt.Errorf("failed to create bunker request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return Clip{}, fmt.Errorf("failed to fetch bunker clip: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return Clip{}, fmt.Errorf("failed to read bunker response: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return Clip{}, fmt.Errorf("bunker API returned status %d: %s", res.StatusCode, string(body))
	}

	var clip Clip
	if err := json.Unmarshal(body, &clip); err != nil {
		return Clip{}, fmt.Errorf("failed to unmarshal bunker response: %w", err)
	}

	if clip.Status != "" && clip.Status != "ready" {
		return Clip{}, fmt.Errorf("bunker clip is not ready: %s", clip.Status)
	}
	if clip.Link == "" {
		return Clip{}, errors.New("bunker clip has no video link")
	}

	return clip, nil
}
