package instagram

type XdtShortcodeMedia struct {
	IsXDTGraphMediaInterface string `json:"__isXDTGraphMediaInterface"`
	Typename                 string `json:"__typename"`
	DisplayURL               string `json:"display_url"`
	VideoURL                 string `json:"video_url"`
	EdgeMediaPreviewLike     struct {
		Count int           `json:"count"`
		Edges []interface{} `json:"edges"`
	} `json:"edge_media_preview_like"`
	EdgeMediaToCaption struct {
		Edges []struct {
			Node struct {
				CreatedAt string `json:"created_at"`
				ID        string `json:"id"`
				Text      string `json:"text"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"edge_media_to_caption"`
	EdgeSidecarToChildren struct {
		Edges []struct {
			Node XdtShortcodeMedia `json:"node"`
		} `json:"edges"`
	} `json:"edge_sidecar_to_children"`
	ID      string `json:"id"`
	IsAd    bool   `json:"is_ad"`
	IsVideo bool   `json:"is_video"`
	Owner   struct {
		FullName         string `json:"full_name"`
		ID               string `json:"id"`
		IsEmbedsDisabled bool   `json:"is_embeds_disabled"`
		IsPrivate        bool   `json:"is_private"`
		IsVerified       bool   `json:"is_verified"`
		Username         string `json:"username"`
	} `json:"owner"`
}

type Node struct {
	DisplayURL string `json:"display_url"`
	IsVideo    bool   `json:"is_video"`
	VideoURL   string `json:"video_url"`
}

type GraphResponse struct {
	Data *struct {
		XdtShortcodeMedia *XdtShortcodeMedia `json:"xdt_shortcode_media"`
	} `json:"data"`
	Extensions struct {
		IsFinal bool `json:"is_final"`
	} `json:"extensions"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// PostInfo represents the structure of Instagram post metadata
type PostInfo struct {
	OwnerUsername string `json:"owner_username"`
	OwnerFullname string `json:"owner_fullname"`
	IsVerified    bool   `json:"is_verified"`
	IsPrivate     bool   `json:"is_private"`
	Likes         int    `json:"likes"`
	IsAd          bool   `json:"is_ad"`
	Caption       string `json:"caption"`
}

// MediaDetails represents the media data structure for Instagram posts
type MediaDetails struct {
	Type           string `json:"type"`
	VideoViewCount int    `json:"video_view_count,omitempty"`
	Url            string `json:"url"`
	Thumbnail      string `json:"thumbnail,omitempty"`
}
