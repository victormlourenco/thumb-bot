package facebook

import (
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Response struct {
	AuthorName    string
	AuthorURL     string
	Caption       string
	PostURL       string
	MediaDetails  []MediaDetail
	IsMarketplace bool
	Title         string
	Price         string
	Location      string
}

type MediaDetail struct {
	Type string // "photo" | "video"
	URL  string
}

type listingPhoto struct {
	Image struct {
		URI string `json:"uri"`
	} `json:"image"`
}

var (
	canonicalPattern = regexp.MustCompile(`(?i)<link\s+rel="canonical"\s+href="([^"]+)"`)
	metaDescPattern  = regexp.MustCompile(`(?i)<meta\s+name="description"\s+content="([^"]*)"`)
	ogPattern        = regexp.MustCompile(`(?i)<meta\s+property="(og:[^"]+)"\s+content="([^"]*)"`)
	imageURIPattern  = regexp.MustCompile(`"(?:full_image|viewer_image|image)":\{"uri":("https:\\/\\/scontent[^"]+")`)
)

var skipCaptions = map[string]bool{
	"Related videos": true,
	"Related pages":  true,
	"Related Reels":  true,
}

func Fetch(inputURL string) (Response, error) {
	client, err := newHTTPClient()
	if err != nil {
		return Response{}, err
	}

	htmlBody, finalURL, err := fetchHTML(client, inputURL)
	if err != nil {
		return Response{}, err
	}

	out := parseHTML(htmlBody, finalURL)
	if isLoginWall(htmlBody) && len(out.MediaDetails) == 0 {
		return Response{}, errors.New("facebook post is private or requires login")
	}
	if len(out.MediaDetails) == 0 && out.Caption == "" && out.AuthorName == "" {
		return Response{}, errors.New("could not extract facebook post")
	}
	return out, nil
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

func setCrawlerHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
}

func fetchHTML(client *http.Client, inputURL string) (string, string, error) {
	htmlBody, finalURL, err := doFetch(client, inputURL, setBrowserHeaders)
	if err == nil && hasUsefulContent(htmlBody) && !isLoginWall(htmlBody) {
		return htmlBody, finalURL, nil
	}

	html2, final2, err2 := doFetch(client, inputURL, setCrawlerHeaders)
	if err2 == nil && (hasUsefulContent(html2) || htmlBody == "") {
		return html2, final2, nil
	}

	if err != nil {
		return "", "", err
	}
	return htmlBody, finalURL, nil
}

func doFetch(client *http.Client, inputURL string, headerFn func(*http.Request)) (string, string, error) {
	req, err := http.NewRequest(http.MethodGet, inputURL, nil)
	if err != nil {
		return "", "", err
	}
	headerFn(req)

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return string(body), resp.Request.URL.String(), errors.New("failed facebook request: " + resp.Status)
	}
	return string(body), resp.Request.URL.String(), nil
}

func hasUsefulContent(htmlBody string) bool {
	return strings.Contains(htmlBody, "browser_native_hd_url") ||
		strings.Contains(htmlBody, "browser_native_sd_url") ||
		strings.Contains(htmlBody, `"__typename":"Photo"`) ||
		strings.Contains(htmlBody, `"message":{"text":`) ||
		strings.Contains(htmlBody, `property="og:image"`) ||
		strings.Contains(htmlBody, `"marketplace_listing_title"`) ||
		strings.Contains(htmlBody, `"listing_photos"`)
}

func isLoginWall(htmlBody string) bool {
	return strings.Contains(htmlBody, "Log in or sign up to view")
}

func parseHTML(htmlBody, fallbackURL string) Response {
	if listing, ok := parseMarketplace(htmlBody, fallbackURL); ok {
		return listing
	}

	ogs := parseOGTags(htmlBody)
	authorName, authorURL := extractAuthor(htmlBody)
	if authorName == "" {
		ogAuthor, _ := parseOGTitle(ogs["og:title"])
		authorName = ogAuthor
	}
	if !isProfileURL(authorURL) {
		authorURL = authorURLFromCanonical(firstNonEmpty(ogs["og:url"], canonicalURL(htmlBody), fallbackURL))
	}

	caption := extractCaption(htmlBody, ogs)
	if caption == "" {
		_, caption = parseOGTitle(ogs["og:title"])
	}

	postURL := firstNonEmpty(canonicalURL(htmlBody), ogs["og:url"], fallbackURL)
	postURL = cleanPostURL(postURL)

	var media []MediaDetail
	if videoURL := pickVideoURL(htmlBody); videoURL != "" {
		media = append(media, MediaDetail{Type: "video", URL: videoURL})
	} else {
		for _, imageURL := range extractPhotoURLs(htmlBody, ogs["og:image"]) {
			media = append(media, MediaDetail{Type: "photo", URL: imageURL})
		}
	}

	return Response{
		AuthorName:   authorName,
		AuthorURL:    authorURL,
		Caption:      caption,
		PostURL:      postURL,
		MediaDetails: media,
	}
}

func isMarketplacePage(htmlBody, pageURL string) bool {
	if strings.Contains(pageURL, "/marketplace/item/") {
		return true
	}
	return strings.Contains(htmlBody, `"marketplace_listing_title"`) && strings.Contains(htmlBody, `"listing_photos"`)
}

func marketplaceDetailsWindow(htmlBody string) string {
	i := strings.Index(htmlBody, `"redacted_description"`)
	if i < 0 {
		return ""
	}
	to := i + 3000
	if to > len(htmlBody) {
		to = len(htmlBody)
	}
	return htmlBody[i:to]
}

func parseMarketplace(htmlBody, fallbackURL string) (Response, bool) {
	if !isMarketplacePage(htmlBody, fallbackURL) {
		return Response{}, false
	}

	ogs := parseOGTags(htmlBody)
	postURL := firstNonEmpty(canonicalURL(htmlBody), ogs["og:url"], fallbackURL)
	postURL = cleanPostURL(postURL)

	details := marketplaceDetailsWindow(htmlBody)
	title := firstNonEmpty(
		firstJSONString(details, "base_marketplace_listing_title"),
		firstJSONString(details, "marketplace_listing_title"),
		ogs["og:title"],
	)
	if i := strings.Index(title, " | "); i >= 0 {
		title = strings.TrimSpace(title[:i])
	}

	caption := nestedJSONText(details, "redacted_description")
	if caption == "" {
		caption = nestedJSONText(htmlBody, "redacted_description")
	}
	if caption == "" {
		caption = firstNonEmpty(ogs["og:description"], metaDescription(htmlBody))
	}

	price := firstNonEmpty(
		firstJSONString(details, "formatted_amount_zeros_stripped"),
		firstJSONString(htmlBody, "formatted_amount_zeros_stripped"),
	)
	location := firstNonEmpty(
		nestedJSONText(details, "location_text"),
		nestedJSONText(htmlBody, "location_text"),
	)

	var media []MediaDetail
	for _, imageURL := range extractListingPhotos(htmlBody) {
		media = append(media, MediaDetail{Type: "photo", URL: imageURL})
	}
	if len(media) == 0 && ogs["og:image"] != "" {
		media = append(media, MediaDetail{Type: "photo", URL: html.UnescapeString(ogs["og:image"])})
	}

	if title == "" && caption == "" && len(media) == 0 {
		return Response{}, false
	}

	return Response{
		AuthorName:    title,
		AuthorURL:     postURL,
		Caption:       caption,
		PostURL:       postURL,
		MediaDetails:  media,
		IsMarketplace: true,
		Title:         title,
		Price:         price,
		Location:      location,
	}, true
}

func nestedJSONText(htmlBody, key string) string {
	needle := `"` + key + `":{"text":`
	i := strings.Index(htmlBody, needle)
	if i < 0 {
		return ""
	}
	text, err := readJSONString(htmlBody[i+len(needle):])
	if err != nil {
		return ""
	}
	return text
}

func metaDescription(htmlBody string) string {
	m := metaDescPattern.FindStringSubmatch(htmlBody)
	if len(m) < 2 {
		return ""
	}
	return html.UnescapeString(m[1])
}

func extractListingPhotos(htmlBody string) []string {
	i := strings.Index(htmlBody, `"listing_photos":`)
	if i < 0 {
		return nil
	}
	rest := strings.TrimSpace(htmlBody[i+len(`"listing_photos":`):])
	raw, err := readJSONValue(rest)
	if err != nil {
		return nil
	}
	var photos []listingPhoto
	if err := json.Unmarshal(raw, &photos); err != nil {
		return nil
	}

	seen := map[string]struct{}{}
	var urls []string
	for _, photo := range photos {
		u := strings.TrimSpace(photo.Image.URI)
		if u == "" || isTinyImage(u) {
			continue
		}
		key := mediaDedupeKey(u)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		urls = append(urls, u)
		if len(urls) == 10 {
			break
		}
	}
	return urls
}

func readJSONValue(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("empty json value")
	}
	switch s[0] {
	case '{', '[':
		depth := 0
		inString := false
		escape := false
		for i := 0; i < len(s); i++ {
			c := s[i]
			if inString {
				if escape {
					escape = false
					continue
				}
				if c == '\\' {
					escape = true
					continue
				}
				if c == '"' {
					inString = false
				}
				continue
			}
			switch c {
			case '"':
				inString = true
			case '{', '[':
				depth++
			case '}', ']':
				depth--
				if depth == 0 {
					return []byte(s[:i+1]), nil
				}
			}
		}
		return nil, errors.New("unterminated json value")
	default:
		return nil, errors.New("unsupported json value")
	}
}

func parseOGTags(htmlBody string) map[string]string {
	out := map[string]string{}
	for _, m := range ogPattern.FindAllStringSubmatch(htmlBody, -1) {
		out[m[1]] = html.UnescapeString(m[2])
	}
	return out
}

func canonicalURL(htmlBody string) string {
	m := canonicalPattern.FindStringSubmatch(htmlBody)
	if len(m) < 2 {
		return ""
	}
	return html.UnescapeString(m[1])
}

func extractAuthor(htmlBody string) (string, string) {
	var name, profile string
	marker := `"__isActor":"User"`
	start := 0
	for {
		i := strings.Index(htmlBody[start:], marker)
		if i < 0 {
			break
		}
		i += start
		from := i - 500
		if from < 0 {
			from = 0
		}
		to := i + 500
		if to > len(htmlBody) {
			to = len(htmlBody)
		}
		window := htmlBody[from:to]
		n := firstJSONString(window, "name")
		u := firstJSONString(window, "url")
		if n != "" && isProfileURL(u) {
			return n, u
		}
		if n != "" && name == "" {
			name = n
			if isProfileURL(u) {
				profile = u
			}
		}
		start = i + len(marker)
	}
	return name, profile
}

func extractCaption(htmlBody string, ogs map[string]string) string {
	needle := `"message":{"text":`
	start := 0
	for {
		i := strings.Index(htmlBody[start:], needle)
		if i < 0 {
			break
		}
		i += start
		rest := htmlBody[i+len(needle):]
		text, err := readJSONString(rest)
		if err == nil && text != "" && !skipCaptions[text] {
			return text
		}
		start = i + len(needle)
	}
	if desc := ogs["og:description"]; desc != "" {
		return desc
	}
	if m := metaDescPattern.FindStringSubmatch(htmlBody); len(m) > 1 {
		return html.UnescapeString(m[1])
	}
	return ""
}

func pickVideoURL(htmlBody string) string {
	keys := []string{
		"browser_native_hd_url",
		"playable_url_quality_hd",
		"browser_native_sd_url",
		"playable_url",
		"hd_src",
		"sd_src",
	}
	var lookaside string
	for _, key := range keys {
		u := nthJSONString(htmlBody, key, 0)
		if u == "" {
			continue
		}
		if strings.Contains(u, "lookaside.fbsbx.com") {
			if lookaside == "" {
				lookaside = u
			}
			continue
		}
		if strings.Contains(u, "fbcdn.net") || strings.Contains(u, ".mp4") {
			return u
		}
	}
	return lookaside
}

func extractPhotoURLs(htmlBody, ogImage string) []string {
	seen := map[string]struct{}{}
	var urls []string

	add := func(u string) {
		u = strings.TrimSpace(u)
		if u == "" || isTinyImage(u) {
			return
		}
		key := mediaDedupeKey(u)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		urls = append(urls, u)
	}

	for _, m := range imageURIPattern.FindAllStringSubmatch(htmlBody, -1) {
		u, err := unmarshalJSONString(m[1])
		if err == nil {
			add(u)
		}
	}
	if len(urls) == 0 && ogImage != "" {
		add(ogImage)
	}
	if len(urls) > 10 {
		urls = urls[:10]
	}
	return urls
}

func isTinyImage(u string) bool {
	tiny := []string{"ctp=s32x32", "ctp=s40x40", "ctp=s50x50", "ctp=s80x80", "s32x32", "s40x40"}
	for _, t := range tiny {
		if strings.Contains(u, t) {
			return true
		}
	}
	return false
}

func mediaDedupeKey(u string) string {
	parsed, err := url.Parse(u)
	if err != nil {
		return u
	}
	return parsed.Path
}

func parseOGTitle(title string) (author, caption string) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", ""
	}
	parts := strings.Split(title, " | ")
	if len(parts) >= 3 {
		return strings.TrimSpace(parts[len(parts)-1]), strings.TrimSpace(strings.Join(parts[1:len(parts)-1], " | "))
	}
	if len(parts) == 2 {
		left := strings.ToLower(parts[0])
		if strings.Contains(left, "views") || strings.Contains(left, "reactions") || strings.Contains(left, "visualiza") {
			return "", strings.TrimSpace(parts[1])
		}
		return strings.TrimSpace(parts[1]), strings.TrimSpace(parts[0])
	}
	return "", title
}

func isProfileURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return false
	}
	path := strings.Trim(parsed.Path, "/")
	if path == "" {
		return false
	}
	parts := strings.Split(path, "/")
	switch parts[0] {
	case "reel", "reels", "watch", "photo", "photos", "videos", "posts", "share", "groups",
		"permalink.php", "story.php", "video.php", "watchparty", "events", "hashtag", "marketplace":
		return false
	case "people":
		return len(parts) >= 3
	case "profile.php":
		return parsed.Query().Get("id") != ""
	default:
		return len(parts) == 1
	}
}

func authorURLFromCanonical(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	switch parts[0] {
	case "reel", "reels", "watch", "photo", "share", "permalink.php", "story.php", "video.php", "marketplace":
		return ""
	case "people":
		if len(parts) >= 3 {
			return "https://www.facebook.com/people/" + parts[1] + "/" + parts[2]
		}
		return ""
	case "groups":
		if len(parts) >= 2 {
			return "https://www.facebook.com/groups/" + parts[1]
		}
		return ""
	default:
		return "https://www.facebook.com/" + parts[0]
	}
}

func cleanPostURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	keep := url.Values{}
	q := parsed.Query()
	for _, k := range []string{"v", "fbid", "story_fbid", "id"} {
		if q.Get(k) != "" {
			keep.Set(k, q.Get(k))
		}
	}
	parsed.RawQuery = keep.Encode()
	parsed.Fragment = ""
	return parsed.String()
}

func firstJSONString(htmlBody, key string) string {
	return nthJSONString(htmlBody, key, 0)
}

func nthJSONString(htmlBody, key string, n int) string {
	needle := `"` + key + `":`
	start := 0
	seen := 0
	for {
		i := strings.Index(htmlBody[start:], needle)
		if i < 0 {
			return ""
		}
		i += start
		rest := strings.TrimSpace(htmlBody[i+len(needle):])
		if strings.HasPrefix(rest, "null") {
			start = i + len(needle)
			continue
		}
		val, err := readJSONString(rest)
		if err != nil {
			start = i + len(needle)
			continue
		}
		if seen == n {
			return val
		}
		seen++
		start = i + len(needle)
	}
}

func readJSONString(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || s[0] != '"' {
		return "", errors.New("not a json string")
	}
	i := 1
	for i < len(s) {
		if s[i] == '\\' {
			i += 2
			continue
		}
		if s[i] == '"' {
			return unmarshalJSONString(s[:i+1])
		}
		i++
	}
	return "", errors.New("unterminated json string")
}

func unmarshalJSONString(quoted string) (string, error) {
	var out string
	if err := json.Unmarshal([]byte(quoted), &out); err != nil {
		return "", err
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
