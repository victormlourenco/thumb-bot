package service

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"thumb-bot/integration/fxtwitter"
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

			// Try fxtwitter first
			fxResponse, fxErr := fxtwitter.Fetch(twUrl.Path)
			if fxErr == nil && fxResponse.Code == 200 {
				t.logger.Info("using fxtwitter provider")
				return t.processFxtwitterResponse(update, fxResponse)
			}

			// Fallback to vxtwitter
			t.logger.Info("fxtwitter failed, trying vxtwitter", zap.Error(fxErr))
			vxResponse, vxErr := vxtwitter.Fetch(twUrl.Path)
			if vxErr != nil {
				t.logger.Error("both fxtwitter and vxtwitter failed", zap.Error(vxErr))
				return vxErr
			}

			t.logger.Info("using vxtwitter provider")
			return t.processVxtwitterResponse(update, vxResponse)
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

func (t *TelegramChannelImpl) processFxtwitterResponse(update telego.Update, response fxtwitter.Response) error {
	if response.Tweet.Media != nil && len(response.Tweet.Media.All) > 0 {
		var mediaGroup []telego.InputMedia
		for i, media := range response.Tweet.Media.All {
			// Use the new variant selection logic
			bestUrl, mediaType, found := fxtwitter.GetBestMediaForTelegram(media)
			if !found {
				continue
			}

			mediaUrl := utils.RemoveQueryParams(bestUrl)
			caption := ""
			if i == 0 {
				caption = fmt.Sprintf("%s\n\n%s: %s\n\n💟 %d 🔁 %d", response.Tweet.URL, response.Tweet.Author.ScreenName, response.Tweet.Text, response.Tweet.Likes, response.Tweet.Retweets)
			}

			switch mediaType {
			case "video":
				mediaGroup = append(mediaGroup, &telego.InputMediaVideo{
					Media:     telego.InputFile{URL: mediaUrl},
					Caption:   caption,
					ParseMode: "HTML",
					Type:      "video",
				})
			case "photo":
				mediaGroup = append(mediaGroup, &telego.InputMediaPhoto{
					Media:     telego.InputFile{URL: mediaUrl},
					Caption:   caption,
					ParseMode: "HTML",
					Type:      "photo",
				})
			}
		}

		if len(mediaGroup) > 0 {
			_, err := t.bot.SendMediaGroup(&telego.SendMediaGroupParams{
				ChatID:           telego.ChatID{ID: update.Message.Chat.ID},
				Media:            mediaGroup,
				ReplyToMessageID: update.Message.MessageID,
			})
			if err != nil {
				t.logger.Error("failed to send media group", zap.Error(err))
				return err
			}
		}
	} else if response.Tweet.Text != "" {
		message := fmt.Sprintf("%s\n\n%s: %s", response.Tweet.URL, response.Tweet.Author.ScreenName, response.Tweet.Text)
		_, err := t.bot.SendMessage(&telego.SendMessageParams{
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
	return nil
}

func (t *TelegramChannelImpl) processVxtwitterResponse(update telego.Update, response vxtwitter.Response) error {
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
			_, err := t.bot.SendMediaGroup(&telego.SendMediaGroupParams{
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
		message := fmt.Sprintf("%s\n\n%s: %s", response.TweetURL, response.UserScreenName, response.Text)
		_, err := t.bot.SendMessage(&telego.SendMessageParams{
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
	return nil
}
