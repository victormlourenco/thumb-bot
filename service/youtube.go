package service

import (
	"fmt"
	"net/url"
	"thumb-bot/integration/youtube"
	"thumb-bot/utils"

	"github.com/mymmrac/telego"
	"go.uber.org/zap"
)

var youtubeHosts = []string{
	"youtube.com",
	"www.youtube.com",
	"m.youtube.com",
	"youtu.be",
	"www.youtu.be",
}

func formatYouTubeCaption(response youtube.YouTubeResponse, videoURL string) string {
	title := escapeText(response.Title)
	if title == "" {
		title = videoURL
	}

	if response.AuthorName == "" {
		return fmt.Sprintf(`<b><a href="%s">%s</a></b>`, escapeText(videoURL), title)
	}

	authorURL := response.AuthorURL
	if authorURL == "" {
		authorURL = "https://www.youtube.com"
	}

	return fmt.Sprintf(
		`<b><a href="%s">%s</a></b>:`+"\n"+`<b><a href="%s">%s</a></b>`,
		escapeText(authorURL),
		escapeText(response.AuthorName),
		escapeText(videoURL),
		title,
	)
}

func (t *TelegramChannelImpl) processYouTubeMedia(update telego.Update) error {
	if update.Message == nil || update.Message.Text == "" {
		return nil
	}

	payload := update.Message.Text
	links := utils.ExtractLinks(payload)
	if len(links) == 0 {
		return nil
	}

	youtubeURL, err := url.Parse(links[0])
	if err != nil {
		t.logger.Error("failed to parse youtube URL", zap.Error(err))
		return err
	}

	isYouTube := false
	for _, host := range youtubeHosts {
		if youtubeURL.Host == host {
			isYouTube = true
			break
		}
	}
	if !isYouTube {
		return nil
	}

	t.logger.Info("fetching YouTube video", zap.String("youtubeURL", youtubeURL.String()))

	response, err := youtube.Fetch(youtubeURL.String())
	if err != nil {
		t.logger.Error("failed to fetch YouTube video", zap.Error(err))
		return err
	}

	directLink, err := youtube.GetDirectLink(youtubeURL.String())
	if err != nil {
		t.logger.Warn("failed to get direct link, using original URL", zap.Error(err))
		directLink = utils.RemoveQueryParams(youtubeURL.String())
	}

	caption := formatYouTubeCaption(response, directLink)
	keyboard := openYouTubeKeyboard(directLink)
	chatID := telego.ChatID{ID: update.Message.Chat.ID}
	replyTo := update.Message.MessageID

	if response.ThumbnailURL != "" {
		_, err := t.bot.SendPhoto(&telego.SendPhotoParams{
			ChatID:           chatID,
			Photo:            telego.InputFile{URL: response.ThumbnailURL},
			Caption:          caption,
			ParseMode:        "HTML",
			ReplyToMessageID: replyTo,
			ReplyMarkup:      keyboard,
		})
		if err != nil {
			t.logger.Error("failed to send YouTube thumbnail", zap.Error(err))
			return err
		}
		return nil
	}

	_, err = t.bot.SendMessage(&telego.SendMessageParams{
		ChatID:                chatID,
		Text:                  caption,
		ParseMode:             "HTML",
		DisableWebPagePreview: true,
		ReplyToMessageID:      replyTo,
		ReplyMarkup:           keyboard,
	})
	if err != nil {
		t.logger.Error("failed to send YouTube message", zap.Error(err))
		return err
	}
	return nil
}
