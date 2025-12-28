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

	// Check if it's a YouTube URL
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

	// Fetch video information
	response, err := youtube.Fetch(youtubeURL.String())
	if err != nil {
		t.logger.Error("failed to fetch YouTube video", zap.Error(err))
		return err
	}

	// Get direct link (normalized YouTube URL)
	directLink, err := youtube.GetDirectLink(youtubeURL.String())
	if err != nil {
		t.logger.Warn("failed to get direct link, using original URL", zap.Error(err))
		directLink = utils.RemoveQueryParams(youtubeURL.String())
	}

	// Create caption with title, author, description, and direct link
	caption := fmt.Sprintf("%s\n\n%s", directLink, response.Title)
	if response.AuthorName != "" {
		caption = fmt.Sprintf("%s\n\n%s: %s", directLink, response.AuthorName, response.Title)
	}
	if response.Description != "" {
		// Truncate description if too long (Telegram has a 1024 character limit for captions)
		description := response.Description
		maxDescLength := 500 // Leave room for title, author, and link
		if len(description) > maxDescLength {
			description = description[:maxDescLength] + "..."
		}
		caption = fmt.Sprintf("%s\n\n%s", caption, description)
	}

	// Send thumbnail as photo with caption
	if response.ThumbnailURL != "" {
		_, err := t.bot.SendPhoto(&telego.SendPhotoParams{
			ChatID:           telego.ChatID{ID: update.Message.Chat.ID},
			Photo:            telego.InputFile{URL: response.ThumbnailURL},
			Caption:          caption,
			ParseMode:        "HTML",
			ReplyToMessageID: update.Message.MessageID,
		})
		if err != nil {
			t.logger.Error("failed to send YouTube thumbnail", zap.Error(err))
			return err
		}
	} else {
		// Fallback: send text message if no thumbnail
		_, err := t.bot.SendMessage(&telego.SendMessageParams{
			ChatID:           telego.ChatID{ID: update.Message.Chat.ID},
			Text:             caption,
			ParseMode:        "HTML",
			ReplyToMessageID: update.Message.MessageID,
		})
		if err != nil {
			t.logger.Error("failed to send YouTube message", zap.Error(err))
			return err
		}
	}

	return nil
}
