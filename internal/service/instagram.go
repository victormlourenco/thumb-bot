package service

import (
	"fmt"
	"net/url"
	"strings"
	"thumb-bot/internal/integration/instagram"
	"thumb-bot/internal/utils"

	"go.uber.org/zap"
	tb "gopkg.in/telebot.v3"
)

var instagramHosts = []string{
	"instagram.com",
	"www.instagram.com",
}

func (t *TelegramChannelImpl) processInstagramMedia(c tb.Context) error {
	payload := c.Message().Text
	links := utils.ExtractLinks(payload)
	if len(links) == 0 {
		return nil
	}

	instaUrl, err := url.Parse(links[0])
	if err != nil {
		t.logger.Error("failed to parse instaUrl", zap.Error(err))
		return err
	}

	for _, host := range instagramHosts {
		if instaUrl.Host == host {

			if strings.Contains(instaUrl.String(), "/stories") {
				return nil
			}

			t.logger.Info("fetching instagram post", zap.String("instaUrl", instaUrl.String()))
			response, err := instagram.GetURL(instaUrl.Path)
			if err != nil {
				t.logger.Error("failed to instagram post", zap.Error(err))
				return err
			}

			if len(response.MediaDetails) > 0 {
				formatedUrl := utils.RemoveQueryParams(instaUrl.String())
				album := tb.Album{}

				caption := fmt.Sprintf("%s\n\n%s", formatedUrl, response.PostInfo.OwnerUsername)
				if len(response.PostInfo.Caption) > 0 {
					caption = fmt.Sprintf("%s\n\n%s: %s", formatedUrl, response.PostInfo.OwnerUsername, response.PostInfo.Caption)
				}

				media := response.MediaDetails[0]
				switch media.Type {
				case "video":
					album = append(album, &tb.Video{File: tb.FromURL(media.URL), Caption: caption})
				case "image":
					album = append(album, &tb.Photo{File: tb.FromURL(media.URL), Caption: caption})
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
			}
		}
	}
	return nil
}
