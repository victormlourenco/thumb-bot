package service

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"thumb-bot/integration/vxtwitter"
	"thumb-bot/utils"

	"github.com/mymmrac/telego"
	"go.uber.org/zap"
)

var twitterHosts = []string{
	"twitter.com",
	"mobile.twitter.com",
	"www.twitter.com",
	"t.co",
	"x.com",
	"www.x.com",
}

func (t *TelegramChannelImpl) processTwitterMedia(update telego.Update) error {
	if update.Message == nil || update.Message.Text == "" {
		return nil
	}

	payload := update.Message.Text
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
				var mediaGroup []telego.InputMedia
				for i, media := range response.MediaExtended {
					mediaUrl := utils.RemoveQueryParams(media.URL)
					caption := ""
					if i == 0 {
						caption = fmt.Sprintf("%s\n\n%s: %s\n\n💟 %d 🔁 %d", response.TweetURL, response.UserScreenName, response.Text, response.Likes, response.Retweets)
					}
					switch media.Type {
					case "video":
						mediaGroup = append(mediaGroup, &telego.InputMediaVideo{
							Media:     telego.InputFile{URL: mediaUrl},
							Caption:   caption,
							ParseMode: "HTML",
							Type:      "video",
						})
					case "image":
						mediaGroup = append(mediaGroup, &telego.InputMediaPhoto{
							Media:     telego.InputFile{URL: mediaUrl},
							Caption:   caption,
							ParseMode: "HTML",
							Type:      "photo",
						})
					}
				}

				if len(mediaGroup) > 0 {
					// Send media group using the bot instance
					_, err = t.bot.SendMediaGroup(&telego.SendMediaGroupParams{
						ChatID:           telego.ChatID{ID: update.Message.Chat.ID},
						Media:            mediaGroup,
						ReplyToMessageID: update.Message.MessageID,
					})
					if err != nil {
						t.logger.Error("failed to send media group", zap.Error(err))
						return err
					}
				}
			} else if response.Text != "" {
				// Send text message using the bot instance
				message := fmt.Sprintf("%s\n\n%s: %s", response.TweetURL, response.UserScreenName, response.Text)
				_, err = t.bot.SendMessage(&telego.SendMessageParams{
					ChatID:           telego.ChatID{ID: update.Message.Chat.ID},
					Text:             message,
					ParseMode:        "HTML",
					ReplyToMessageID: update.Message.MessageID,
				})
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
