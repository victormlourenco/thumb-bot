package fxtwitter

// MediaExtended represents the media data structure for fxtwitter
type MediaExtended struct {
	URL          string  `json:"url"`
	ThumbnailURL string  `json:"thumbnail_url"`
	Duration     float64 `json:"duration"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	Format       string  `json:"format"`
	Type         string  `json:"type"`
}

// AuthorInfo represents the author information for fxtwitter
type AuthorInfo struct {
	Name       string `json:"name"`
	ScreenName string `json:"screen_name"`
	AvatarURL  string `json:"avatar_url"`
	Followers  int    `json:"followers"`
	Following  int    `json:"following"`
	Verified   bool   `json:"verified"`
}

// TweetInfo represents the tweet metadata for fxtwitter
type TweetInfo struct {
	URL      string `json:"url"`
	ID       string `json:"id"`
	Text     string `json:"text"`
	Likes    int    `json:"likes"`
	Retweets int    `json:"retweets"`
	Replies  int    `json:"replies"`
	Views    int    `json:"views"`
}
