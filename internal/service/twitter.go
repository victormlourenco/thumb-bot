package service

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"thumb-bot/internal/integration/vxtwitter"
	"thumb-bot/internal/utils"

	"go.uber.org/zap"
	tb "gopkg.in/telebot.v3"
)

var twitterHosts = []string{
	"twitter.com",
	"mobile.twitter.com",
	"www.twitter.com",
	"t.co",
	"x.com",
	"www.x.com",
}

func (t *TelegramChannelImpl) processTwitterMedia(c tb.Context) error {
	payload := c.Message().Text
	links := utils.ExtractLinks(payload)
	if len(links) == 0 {
		return nil
	}

	if strings.Contains(links[0], "t.co") {
		links[0], _ = expandShortURL(links[0])
	}

	twUrl, err := url.Parse(links[0])
	if err != nil {
		t.logger.Error("failed to parse twUrl", zap.Error(err))
		return err
	}

	for _, host := range twitterHosts {
		if twUrl.Host == host {
			t.logger.Info("fetching tweet", zap.String("twUrl", twUrl.String()))
			response, err := vxtwitter.Fetch(twUrl.Path)
			if err != nil {
				t.logger.Error("failed to fetch tweet", zap.Error(err))
				return err
			}

			if len(response.MediaExtended) > 0 {
				album := tb.Album{}
				for _, media := range response.MediaExtended {
					mediaUrl := utils.RemoveQueryParams(media.URL)
					caption := fmt.Sprintf("%s\n\n%s: %s\n\n💟 %d 🔁 %d", response.TweetURL, response.UserScreenName, response.Text, response.Likes, response.Retweets)
					switch media.Type {
					case "video":
						album = append(album, &tb.Video{File: tb.FromURL(mediaUrl), Caption: caption})
					case "image":
						album = append(album, &tb.Photo{File: tb.FromURL(mediaUrl), Caption: caption})
					}
				}

				if len(album) > 0 {
					options := &tb.SendOptions{
						ReplyTo: c.Message(),
					}
					err = c.SendAlbum(album, options)
					if err != nil {
						t.logger.Error("failed to send album", zap.Error(err))
						return err
					}
				}
			} else if response.Text != "" {
				options := &tb.SendOptions{
					ReplyTo: c.Message(),
				}
				message := fmt.Sprintf("%s\n\n%s: %s", response.TweetURL, response.UserScreenName, response.Text)
				err := c.Send(message, options)
				if err != nil {
					t.logger.Error("failed to send message", zap.Error(err))
					return err
				}
			}
		}
	}
	return nil
}

func expandShortURL(shortURL string) (string, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Head(shortURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return resp.Header.Get("Location"), nil
}
