package service

import (
	"fmt"
	"net/url"
	"strings"
	"thumb-bot/integration/instagram"
	"thumb-bot/utils"

	"github.com/mymmrac/telego"
	"go.uber.org/zap"
)

var instagramHosts = []string{
	"instagram.com",
	"www.instagram.com",
}

func (t *TelegramChannelImpl) processInstagramMedia(update telego.Update) error {
	if update.Message == nil || update.Message.Text == "" {
		return nil
	}

	payload := update.Message.Text
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
				var mediaGroup []telego.InputMedia

				caption := fmt.Sprintf("%s\n\n%s", formatedUrl, response.PostInfo.OwnerUsername)
				if len(response.PostInfo.Caption) > 0 {
					caption = fmt.Sprintf("%s\n\n%s: %s", formatedUrl, response.PostInfo.OwnerUsername, response.PostInfo.Caption)
				}

				media := response.MediaDetails[0]
				switch media.Type {
				case "video":
					mediaGroup = append(mediaGroup, &telego.InputMediaVideo{
						Media:     telego.InputFile{URL: media.URL},
						Caption:   caption,
						ParseMode: "HTML",
						Type:      "video",
					})
				case "image":
					mediaGroup = append(mediaGroup, &telego.InputMediaPhoto{
						Media:     telego.InputFile{URL: media.URL},
						Caption:   caption,
						ParseMode: "HTML",
						Type:      "photo",
					})
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
			}
		}
	}
	return nil
}
