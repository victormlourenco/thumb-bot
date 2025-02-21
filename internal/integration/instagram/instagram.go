package instagram

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// checkRedirect checks if the URL includes "share" and fetches the actual path
func checkRedirect(url string) (string, error) {
	splitUrl := strings.Split(url, "/")
	if contains(splitUrl, "share") {
		resp, err := http.Get(url)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		return resp.Request.URL.Path, nil
	}
	return url, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// formatPostInfo formats the post information from the response data
func formatPostInfo(requestData XdtShortcodeMedia) (PostInfo, error) {
	owner := requestData.Owner
	edgeMediaToCaption := requestData.EdgeMediaToCaption.Edges
	var caption string
	if len(edgeMediaToCaption) > 0 {
		caption = edgeMediaToCaption[0].Node.Text
	}

	return PostInfo{
		OwnerUsername: owner.Username,
		OwnerFullname: owner.FullName,
		IsVerified:    owner.IsVerified,
		IsPrivate:     owner.IsPrivate,
		Likes:         requestData.EdgeMediaPreviewLike.Count,
		IsAd:          requestData.IsAd,
		Caption:       caption,
	}, nil
}

// formatMediaDetails formats media data based on whether it's video or image
func formatMediaDetails(mediaData XdtShortcodeMedia) (MediaDetails, error) {
	if mediaData.IsVideo {
		return MediaDetails{
			Type:      "video",
			Url:       formatMediaUrl(mediaData.VideoURL),
			Thumbnail: formatMediaUrl(mediaData.DisplayURL),
		}, nil
	} else {
		return MediaDetails{
			Type:      "image",
			Url:       formatMediaUrl(mediaData.DisplayURL),
			Thumbnail: formatMediaUrl(mediaData.DisplayURL),
		}, nil
	}
}

// getShortcode extracts the shortcode from the URL
func getShortcode(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}
	path := strings.Split(u.Path, "/")
	for i, segment := range path {
		if contains([]string{"p", "reel", "tv", "reels"}, segment) && i+1 < len(path) {
			return path[i+1], nil
		}
	}
	return "", errors.New("could not find shortcode in url")
}

// isSidecar checks if the post is actually a sidecar (carousel post)
func isSidecar(requestData XdtShortcodeMedia) bool {
	return requestData.Typename == "XDTGraphSidecar"
}

// instagramRequest makes an API request to Instagram's GraphQL endpoint
func instagramRequest(shortcode string) (XdtShortcodeMedia, error) {
	data := url.Values{}
	data.Set("variables", fmt.Sprintf(`{"shortcode":"%s","fetch_tagged_user_count":null,"hoisted_comment_id":null,"hoisted_reply_id":null}`, shortcode))
	data.Set("doc_id", "8845758582119845")

	resp, err := http.PostForm("https://www.instagram.com/graphql/query", data)
	if err != nil {
		return XdtShortcodeMedia{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return XdtShortcodeMedia{}, err
	}

	result := GraphResponse{}
	if err := json.Unmarshal(body, &result); err != nil {
		return XdtShortcodeMedia{}, err
	}

	if result.Data == nil || result.Data.XdtShortcodeMedia == nil {
		return XdtShortcodeMedia{}, fmt.Errorf("failed getting XDT short code: %s", result.Message)
	}

	return *result.Data.XdtShortcodeMedia, nil
}

// OutputData represents the final structured output
type OutputData struct {
	ResultsNumber int            `json:"results_number"`
	UrlList       []string       `json:"url_list"`
	PostInfo      PostInfo       `json:"post_info"`
	MediaDetails  []MediaDetails `json:"media_details"`
}

// createOutputData constructs the final response from Instagram data
func createOutputData(requestData XdtShortcodeMedia) (OutputData, error) {
	var urlList []string
	var mediaDetails []MediaDetails

	isSidecar := isSidecar(requestData)
	if isSidecar {
		edges := requestData.EdgeSidecarToChildren.Edges
		for _, edge := range edges {
			node := edge.Node
			media, err := formatMediaDetails(node)
			if err != nil {
				return OutputData{}, err
			}
			mediaDetails = append(mediaDetails, media)
			if node.IsVideo {
				urlList = append(urlList, node.VideoURL)
			} else {
				urlList = append(urlList, node.DisplayURL)
			}
		}
	} else {
		media, err := formatMediaDetails(requestData)
		if err != nil {
			return OutputData{}, err
		}
		mediaDetails = append(mediaDetails, media)
		if requestData.IsVideo {
			urlList = append(urlList, requestData.VideoURL)
		} else {
			urlList = append(urlList, requestData.DisplayURL)
		}
	}

	postInfo, err := formatPostInfo(requestData)
	if err != nil {
		return OutputData{}, err
	}

	return OutputData{
		ResultsNumber: len(urlList),
		UrlList:       urlList,
		PostInfo:      postInfo,
		MediaDetails:  mediaDetails,
	}, nil
}

// GetUrl fetches and processes an Instagram URL to return formatted data
func GetUrl(urlMedia string) (OutputData, error) {
	redirectedUrl, err := checkRedirect(urlMedia)
	if err != nil {
		return OutputData{}, err
	}

	shortcode, err := getShortcode(redirectedUrl)
	if err != nil {
		return OutputData{}, err
	}

	instagramData, err := instagramRequest(shortcode)
	if err != nil {
		return OutputData{}, err
	}

	return createOutputData(instagramData)
}

func formatMediaUrl(mediaUrl string) string {
	u, err := url.Parse(mediaUrl)
	if err != nil {
		return ""
	}
	u.Host = "scontent.cdninstagram.com"
	return u.String()
}
