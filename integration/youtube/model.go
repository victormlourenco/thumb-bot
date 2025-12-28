package youtube

// YouTubeResponse represents the response from YouTube oEmbed API
type YouTubeResponse struct {
	Title           string `json:"title"`
	AuthorName      string `json:"author_name"`
	AuthorURL       string `json:"author_url"`
	Type            string `json:"type"`
	Height          int    `json:"height"`
	Width           int    `json:"width"`
	Version         string `json:"version"`
	ProviderName    string `json:"provider_name"`
	ProviderURL     string `json:"provider_url"`
	ThumbnailURL    string `json:"thumbnail_url"`
	ThumbnailWidth  int    `json:"thumbnail_width"`
	ThumbnailHeight int    `json:"thumbnail_height"`
	HTML            string `json:"html"`
	Description     string `json:"description,omitempty"`
}
