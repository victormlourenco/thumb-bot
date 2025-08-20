package instagram

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ===== Public types =====

var (
	csrfToken     string
	csrfTokenExp  time.Time
	csrfTokenLock = &sync.Mutex{}
	cfg           = Config{Retries: 5, Delay: time.Second}
)

type InstagramResponse struct {
	ResultsNumber int      `json:"results_number"`
	URLList       []string `json:"url_list"`
	PostInfo      struct {
		OwnerUsername string `json:"owner_username"`
		OwnerFullname string `json:"owner_fullname"`
		IsVerified    bool   `json:"is_verified"`
		IsPrivate     bool   `json:"is_private"`
		Likes         int    `json:"likes"`
		IsAd          bool   `json:"is_ad"`
		Caption       string `json:"caption"`
	} `json:"post_info"`
	MediaDetails []MediaDetail `json:"media_details"`
}

type MediaDetail struct {
	Type           string     `json:"type"` // "image" | "video"
	Dimensions     Dimensions `json:"dimensions"`
	URL            string     `json:"url"`
	VideoViewCount *int       `json:"video_view_count,omitempty"`
	Thumbnail      *string    `json:"thumbnail,omitempty"`
}

type Dimensions struct {
	Height int `json:"height"`
	Width  int `json:"width"`
}

type Config struct {
	Retries int
	Delay   time.Duration // initial delay between retries
}

// ===== Internal structs (map the GraphQL JSON we need) =====

type graphResponse struct {
	Data struct {
		ShortcodeMedia *node `json:"xdt_shortcode_media"`
	} `json:"data"`
}

type node struct {
	Typename             string    `json:"__typename"`
	Owner                owner     `json:"owner"`
	EdgeMediaToCaption   edgesText `json:"edge_media_to_caption"`
	EdgeMediaPreviewLike struct {
		Count int `json:"count"`
	} `json:"edge_media_preview_like"`
	IsAd                  bool       `json:"is_ad"`
	IsVideo               bool       `json:"is_video"`
	Dimensions            Dimensions `json:"dimensions"`
	VideoViewCount        *int       `json:"video_view_count,omitempty"`
	VideoURL              string     `json:"video_url"`
	DisplayURL            string     `json:"display_url"`
	EdgeSidecarToChildren struct {
		Edges []struct {
			Node node `json:"node"`
		} `json:"edges"`
	} `json:"edge_sidecar_to_children"`
}

type owner struct {
	Username   string `json:"username"`
	FullName   string `json:"full_name"`
	IsVerified bool   `json:"is_verified"`
	IsPrivate  bool   `json:"is_private"`
}

type edgesText struct {
	Edges []struct {
		Node struct {
			Text string `json:"text"`
		} `json:"node"`
	} `json:"edges"`
}

// ===== Public API =====

func GetURL(inputURL string) (InstagramResponse, error) {

	client, err := newHTTPClient()
	if err != nil {
		return InstagramResponse{}, err
	}

	// 1) Resolve share redirects if present
	finalURL, err := checkRedirect(client, inputURL)
	if err != nil {
		return InstagramResponse{}, err
	}

	// 2) Extract shortcode
	shortcode, err := getShortcode(finalURL)
	if err != nil {
		return InstagramResponse{}, err
	}

	// 3) Fetch post via GraphQL (with retries/backoff)
	post, err := instagramRequest(client, shortcode, cfg.Retries, cfg.Delay)
	if err != nil {
		return InstagramResponse{}, err
	}

	// 4) Shape output
	out, err := createOutputData(post)
	if err != nil {
		return InstagramResponse{}, err
	}
	return out, nil
}

// ===== Utilities =====

func newHTTPClient() (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}, nil
}

func checkRedirect(client *http.Client, u string) (string, error) {
	// Mimic the TS behavior: if URL contains "share", follow it and return the final URL
	if strings.Contains(u, "/share/") || strings.Contains(u, "/share") {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			return "", err
		}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		// final URL after redirects:
		return resp.Request.URL.String(), nil
	}
	return u, nil
}

func getShortcode(u string) (string, error) {
	parts := strings.Split(u, "/")
	tags := map[string]struct{}{"p": {}, "reel": {}, "tv": {}, "reels": {}}
	for i, p := range parts {
		if _, ok := tags[p]; ok {
			if i+1 < len(parts) && parts[i+1] != "" {
				return parts[i+1], nil
			}
			break
		}
	}
	return "", errors.New("failed to obtain shortcode")
}

func getCSRFToken(client *http.Client) (string, error) {
	csrfTokenLock.Lock()
	defer csrfTokenLock.Unlock()

	// If in memory token is valid, return it
	if csrfToken != "" && time.Now().Before(csrfTokenExp) {
		return csrfToken, nil
	}

	req, err := http.NewRequest(http.MethodGet, "https://www.instagram.com/", nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	for _, c := range resp.Cookies() {
		if c.Name == "csrftoken" && c.Value != "" {
			csrfToken = c.Value
			csrfTokenExp = time.Now().Add(10 * time.Minute) // cache por 10 minutos
			return csrfToken, nil
		}
	}
	// Fallback: busca no header
	if cookies := resp.Header["Set-Cookie"]; len(cookies) > 0 {
		for _, raw := range cookies {
			if strings.HasPrefix(raw, "csrftoken=") {
				semi := strings.Index(raw, ";")
				val := raw[len("csrftoken="):]
				if semi >= 0 {
					val = raw[len("csrftoken="):semi]
				}
				if val != "" {
					csrfToken = val
					csrfTokenExp = time.Now().Add(10 * time.Minute)
					return csrfToken, nil
				}
			}
		}
	}
	return "", errors.New("CSRF token not found in response headers")
}

func instagramRequest(client *http.Client, shortcode string, retries int, delay time.Duration) (*node, error) {
	const baseURL = "https://www.instagram.com/graphql/query"
	const docID = "9510064595728286"

	// 1) CSRF token
	token, err := getCSRFToken(client)
	if err != nil {
		return nil, wrapErr("failed to obtain CSRF", err)
	}

	// 2) Build form body
	variables := map[string]interface{}{
		"shortcode":               shortcode,
		"fetch_tagged_user_count": nil,
		"hoisted_comment_id":      nil,
		"hoisted_reply_id":        nil,
	}
	varJSON, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("variables", string(varJSON))
	form.Set("doc_id", docID)

	req, err := http.NewRequest(http.MethodPost, baseURL, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-CSRFToken", token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Retry on 429 / 403 like TS code
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden {
		if retries > 0 {
			wait := delay
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if sec, convErr := strconv.Atoi(ra); convErr == nil && sec > 0 {
					wait = time.Duration(sec) * time.Second
				}
			}
			time.Sleep(wait)
			// Exponential backoff on the "delay" path
			return instagramRequest(client, shortcode, retries-1, delay*2)
		}
		b, _ := io.ReadAll(resp.Body)
		return nil, errors.New("failed instagram request after retries: " + string(b))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, errors.New("failed instagram request: " + resp.Status + " - " + string(b))
	}

	var gr graphResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return nil, err
	}
	if gr.Data.ShortcodeMedia == nil {
		return nil, errors.New("only posts/reels supported, check if your link is valid")
	}
	return gr.Data.ShortcodeMedia, nil
}

func createOutputData(n *node) (InstagramResponse, error) {
	if n == nil {
		return InstagramResponse{}, errors.New("nil post data")
	}

	var out InstagramResponse

	// Post info
	out.PostInfo.OwnerUsername = n.Owner.Username
	out.PostInfo.OwnerFullname = n.Owner.FullName
	out.PostInfo.IsVerified = n.Owner.IsVerified
	out.PostInfo.IsPrivate = n.Owner.IsPrivate
	out.PostInfo.Likes = n.EdgeMediaPreviewLike.Count
	out.PostInfo.IsAd = n.IsAd
	out.PostInfo.Caption = firstCaption(n.EdgeMediaToCaption)

	// Media
	var urls []string
	var details []MediaDetail

	if isSidecar(n) {
		for _, e := range n.EdgeSidecarToChildren.Edges {
			md := formatMediaDetails(&e.Node)
			details = append(details, md)
			urls = append(urls, mediaURL(&e.Node))
		}
	} else {
		md := formatMediaDetails(n)
		details = append(details, md)
		urls = append(urls, mediaURL(n))
	}

	out.ResultsNumber = len(urls)
	out.URLList = urls
	out.MediaDetails = details
	return out, nil
}

func firstCaption(et edgesText) string {
	if len(et.Edges) == 0 {
		return ""
	}
	return et.Edges[0].Node.Text
}

func isSidecar(n *node) bool {
	return n.Typename == "XDTGraphSidecar"
}

func formatMediaDetails(n *node) MediaDetail {
	if n.IsVideo {
		thumb := n.DisplayURL
		return MediaDetail{
			Type:           "video",
			Dimensions:     n.Dimensions,
			URL:            n.VideoURL,
			VideoViewCount: n.VideoViewCount,
			Thumbnail:      &thumb,
		}
	}
	return MediaDetail{
		Type:       "image",
		Dimensions: n.Dimensions,
		URL:        n.DisplayURL,
	}
}

func mediaURL(n *node) string {
	if n.IsVideo {
		return n.VideoURL
	}
	return n.DisplayURL
}

func wrapErr(msg string, err error) error {
	return errors.New(msg + ": " + err.Error())
}

/*
Example usage:

func main() {
	resp, err := InstagramGetURL("https://www.instagram.com/p/SHORTCODE/", &Config{Retries: 5, Delay: time.Second})
	if err != nil {
		log.Fatal(err)
	}
	b, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println(string(b))
}
*/
