package instagram

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ===== Public types =====

var (
	csrfToken     string
	csrfTokenExp  time.Time
	lsdToken      string
	csrfTokenLock = &sync.Mutex{}
	cfg           = Config{Retries: 5, Delay: time.Second, MaxDelay: 30 * time.Second}
	lsdPattern    = regexp.MustCompile(`"LSD",\[\],\{"token":"([^"]+)"\}`)
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
	Retries  int
	Delay    time.Duration // initial delay between retries
	MaxDelay time.Duration // maximum delay cap for exponential backoff
}

// ===== Internal structs (Polaris / v1 web_info media) =====

type graphResponse struct {
	Data struct {
		WebInfo *webInfo `json:"xdt_api__v1__media__shortcode__web_info"`
	} `json:"data"`
}

type webInfo struct {
	Items []mediaItem `json:"items"`
}

type mediaItem struct {
	Code              string         `json:"code"`
	MediaType         int            `json:"media_type"` // 1=image, 2=video, 8=sidecar
	OriginalWidth     int            `json:"original_width"`
	OriginalHeight    int            `json:"original_height"`
	LikeCount         int            `json:"like_count"`
	ViewCount         *int           `json:"view_count"`
	PlayCount         *int           `json:"play_count"`
	IsPaidPartnership bool           `json:"is_paid_partnership"`
	User              mediaUser      `json:"user"`
	Caption           *mediaCaption  `json:"caption"`
	ImageVersions2    *imageVersions `json:"image_versions2"`
	VideoVersions     []mediaVersion `json:"video_versions"`
	CarouselMedia     []mediaItem    `json:"carousel_media"`
}

type mediaUser struct {
	Username   string `json:"username"`
	FullName   string `json:"full_name"`
	IsVerified bool   `json:"is_verified"`
	IsPrivate  bool   `json:"is_private"`
}

type mediaCaption struct {
	Text string `json:"text"`
}

type imageVersions struct {
	Candidates []mediaVersion `json:"candidates"`
}

type mediaVersion struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type sessionTokens struct {
	csrf string
	lsd  string
}

// ===== Public API =====

func GetURL(inputURL string) (InstagramResponse, error) {
	client, err := newHTTPClient()
	if err != nil {
		return InstagramResponse{}, err
	}

	absoluteURL := normalizeInputURL(inputURL)

	// 1) Resolve share redirects if present
	finalURL, err := checkRedirect(client, absoluteURL)
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

func normalizeInputURL(input string) string {
	input = strings.TrimSpace(input)
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		return input
	}
	if strings.HasPrefix(input, "/") {
		return "https://www.instagram.com" + input
	}
	return "https://www.instagram.com/" + input
}

// setBrowserHeaders adds browser-like headers to mimic a real browser request.
// Do not set Accept-Encoding: Go's transport auto-decompresses when it is unset.
func setBrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Sec-Ch-Ua", `"Google Chrome";v="143", "Chromium";v="143", "Not A(Brand";v="24"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
}

// setGraphQLHeaders adds headers specific to Instagram GraphQL API requests
func setGraphQLHeaders(req *http.Request, tokens sessionTokens) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://www.instagram.com")
	req.Header.Set("Referer", "https://www.instagram.com/")
	req.Header.Set("X-CSRFToken", tokens.csrf)
	req.Header.Set("X-IG-App-ID", "936619743392459")
	req.Header.Set("X-ASBD-ID", "359341")
	req.Header.Set("X-IG-WWW-Claim", "0")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Sec-Ch-Ua", `"Google Chrome";v="143", "Chromium";v="143", "Not A(Brand";v="24"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	if tokens.lsd != "" {
		req.Header.Set("X-FB-LSD", tokens.lsd)
	}
}

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
	if strings.Contains(u, "/share/") || strings.Contains(u, "/share") {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			return "", err
		}
		setBrowserHeaders(req)
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)
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
				code := parts[i+1]
				if q := strings.IndexAny(code, "?#"); q >= 0 {
					code = code[:q]
				}
				if code != "" {
					return code, nil
				}
			}
			break
		}
	}
	return "", errors.New("failed to obtain shortcode")
}

// invalidateCSRFToken clears the cached CSRF token, forcing a refresh on next request
func invalidateCSRFToken() {
	csrfTokenLock.Lock()
	defer csrfTokenLock.Unlock()
	csrfToken = ""
	lsdToken = ""
	csrfTokenExp = time.Time{}
}

func applyCSRFCookie(client *http.Client, token string) {
	u, err := url.Parse("https://www.instagram.com/")
	if err != nil || token == "" {
		return
	}
	client.Jar.SetCookies(u, []*http.Cookie{{
		Name:   "csrftoken",
		Value:  token,
		Path:   "/",
		Domain: ".instagram.com",
	}})
}

func getSessionTokens(client *http.Client) (sessionTokens, error) {
	csrfTokenLock.Lock()
	defer csrfTokenLock.Unlock()

	// Reuse cached tokens, but always seed the current client's cookie jar.
	if csrfToken != "" && time.Now().Before(csrfTokenExp) {
		applyCSRFCookie(client, csrfToken)
		return sessionTokens{csrf: csrfToken, lsd: lsdToken}, nil
	}

	req, err := http.NewRequest(http.MethodGet, "https://www.instagram.com/", nil)
	if err != nil {
		return sessionTokens{}, err
	}
	setBrowserHeaders(req)
	resp, err := client.Do(req)
	if err != nil {
		return sessionTokens{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return sessionTokens{}, err
	}

	token := ""
	for _, c := range resp.Cookies() {
		if c.Name == "csrftoken" && c.Value != "" {
			token = c.Value
			break
		}
	}
	if token == "" {
		u, _ := url.Parse("https://www.instagram.com/")
		for _, c := range client.Jar.Cookies(u) {
			if c.Name == "csrftoken" && c.Value != "" {
				token = c.Value
				break
			}
		}
	}
	if token == "" {
		if cookies := resp.Header["Set-Cookie"]; len(cookies) > 0 {
			for _, raw := range cookies {
				if strings.HasPrefix(raw, "csrftoken=") {
					semi := strings.Index(raw, ";")
					val := raw[len("csrftoken="):]
					if semi >= 0 {
						val = raw[len("csrftoken="):semi]
					}
					if val != "" {
						token = val
						break
					}
				}
			}
		}
	}
	if token == "" {
		return sessionTokens{}, errors.New("CSRF token not found in response headers")
	}

	lsd := ""
	if m := lsdPattern.FindSubmatch(body); m != nil {
		lsd = string(m[1])
	}

	csrfToken = token
	lsdToken = lsd
	csrfTokenExp = time.Now().Add(10 * time.Minute)
	applyCSRFCookie(client, csrfToken)
	return sessionTokens{csrf: csrfToken, lsd: lsdToken}, nil
}

func instagramRequest(client *http.Client, shortcode string, retries int, delay time.Duration) (*mediaItem, error) {
	// PolarisPostRootQuery — legacy xdt_shortcode_media doc_ids were deprecated mid-2026.
	const baseURL = "https://www.instagram.com/graphql/query"
	const docID = "27128499623469141"

	tokens, err := getSessionTokens(client)
	if err != nil {
		return nil, wrapErr("failed to obtain CSRF", err)
	}

	variables := map[string]interface{}{
		"shortcode": shortcode,
		"__relay_internal__pv__PolarisAIGMMediaWebLabelEnabledrelayprovider": false,
	}
	varJSON, err := json.Marshal(variables)
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("variables", string(varJSON))
	form.Set("doc_id", docID)
	if tokens.lsd != "" {
		form.Set("lsd", tokens.lsd)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, err
	}
	setGraphQLHeaders(req, tokens)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		if retries > 0 {
			invalidateCSRFToken()

			wait := delay
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if sec, convErr := strconv.Atoi(ra); convErr == nil && sec > 0 {
					wait = time.Duration(sec) * time.Second
				}
			}

			jitter := time.Duration(float64(wait) * (0.5 - rand.Float64()) * 0.5)
			wait += jitter

			if wait > cfg.MaxDelay {
				wait = cfg.MaxDelay
			}

			time.Sleep(wait)

			nextDelay := delay * 2
			if nextDelay > cfg.MaxDelay {
				nextDelay = cfg.MaxDelay
			}
			return instagramRequest(client, shortcode, retries-1, nextDelay)
		}
		b, _ := io.ReadAll(resp.Body)
		return nil, errors.New("failed instagram request after retries: " + string(b))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, errors.New("failed instagram request: " + resp.Status + " - " + string(b))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var gr graphResponse
	if err := json.Unmarshal(body, &gr); err != nil {
		return nil, err
	}
	if gr.Data.WebInfo == nil || len(gr.Data.WebInfo.Items) == 0 {
		return nil, errors.New("only posts/reels supported, check if your link is valid")
	}
	return &gr.Data.WebInfo.Items[0], nil
}

func createOutputData(item *mediaItem) (InstagramResponse, error) {
	if item == nil {
		return InstagramResponse{}, errors.New("nil post data")
	}

	var out InstagramResponse

	out.PostInfo.OwnerUsername = item.User.Username
	out.PostInfo.OwnerFullname = item.User.FullName
	out.PostInfo.IsVerified = item.User.IsVerified
	out.PostInfo.IsPrivate = item.User.IsPrivate
	out.PostInfo.Likes = item.LikeCount
	out.PostInfo.IsAd = item.IsPaidPartnership
	if item.Caption != nil {
		out.PostInfo.Caption = item.Caption.Text
	}

	var urls []string
	var details []MediaDetail

	if item.MediaType == 8 && len(item.CarouselMedia) > 0 {
		for i := range item.CarouselMedia {
			md := formatMediaDetails(&item.CarouselMedia[i])
			details = append(details, md)
			urls = append(urls, md.URL)
		}
	} else {
		md := formatMediaDetails(item)
		details = append(details, md)
		urls = append(urls, md.URL)
	}

	out.ResultsNumber = len(urls)
	out.URLList = urls
	out.MediaDetails = details
	return out, nil
}

func formatMediaDetails(item *mediaItem) MediaDetail {
	dims := Dimensions{
		Width:  item.OriginalWidth,
		Height: item.OriginalHeight,
	}
	displayURL := bestImageURL(item)
	if displayURL != "" && (dims.Width == 0 || dims.Height == 0) {
		if item.ImageVersions2 != nil && len(item.ImageVersions2.Candidates) > 0 {
			dims.Width = item.ImageVersions2.Candidates[0].Width
			dims.Height = item.ImageVersions2.Candidates[0].Height
		}
	}

	if item.MediaType == 2 || len(item.VideoVersions) > 0 {
		videoURL := bestVideoURL(item)
		thumb := displayURL
		viewCount := item.ViewCount
		if viewCount == nil {
			viewCount = item.PlayCount
		}
		if len(item.VideoVersions) > 0 && (dims.Width == 0 || dims.Height == 0) {
			dims.Width = item.VideoVersions[0].Width
			dims.Height = item.VideoVersions[0].Height
		}
		return MediaDetail{
			Type:           "video",
			Dimensions:     dims,
			URL:            videoURL,
			VideoViewCount: viewCount,
			Thumbnail:      &thumb,
		}
	}

	return MediaDetail{
		Type:       "image",
		Dimensions: dims,
		URL:        displayURL,
	}
}

func bestImageURL(item *mediaItem) string {
	if item.ImageVersions2 == nil || len(item.ImageVersions2.Candidates) == 0 {
		return ""
	}
	return item.ImageVersions2.Candidates[0].URL
}

func bestVideoURL(item *mediaItem) string {
	if len(item.VideoVersions) == 0 {
		return ""
	}
	return item.VideoVersions[0].URL
}

func wrapErr(msg string, err error) error {
	return errors.New(msg + ": " + err.Error())
}
