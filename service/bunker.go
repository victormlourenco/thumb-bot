package service

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"thumb-bot/integration/bunker"
	"thumb-bot/utils"

	"github.com/mymmrac/telego"
	"go.uber.org/zap"
)

var bunkerHosts = []string{
	"bnkr.in",
	"www.bnkr.in",
}

func formatBunkerCaption(clip bunker.Clip) string {
	var parts []string

	if clip.User.Name != "" {
		header := fmt.Sprintf("<b>%s</b>", escapeText(clip.User.Name))
		if clip.Title != "" {
			parts = append(parts, fmt.Sprintf("%s:\n%s", header, escapeText(clip.Title)))
		} else {
			parts = append(parts, header)
		}
	} else if clip.Title != "" {
		parts = append(parts, fmt.Sprintf("<b>%s</b>", escapeText(clip.Title)))
	}

	if clip.Live.Name != "" {
		parts = append(parts, fmt.Sprintf("<i>%s</i>", escapeText(clip.Live.Name)))
	}

	return strings.Join(parts, "\n")
}

func (t *TelegramChannelImpl) processBunkerMedia(update telego.Update) error {
	if update.Message == nil || update.Message.Text == "" {
		return nil
	}

	links := utils.ExtractLinks(update.Message.Text)
	if len(links) == 0 {
		return nil
	}

	clipURL, err := url.Parse(links[0])
	if err != nil {
		t.logger.Error("failed to parse bunker URL", zap.Error(err))
		return err
	}

	isBunker := false
	for _, host := range bunkerHosts {
		if clipURL.Host == host {
			isBunker = true
			break
		}
	}
	if !isBunker {
		return nil
	}

	if _, allowed := t.bunkerAllowedChatIDs[update.Message.Chat.ID]; !allowed {
		t.logger.Info("ignoring bunker clip from unauthorized chat", zap.Int64("chat_id", update.Message.Chat.ID))
		return nil
	}

	clipID, err := bunker.ExtractClipID(clipURL.String())
	if err != nil {
		t.logger.Error("failed to extract bunker clip id", zap.Error(err))
		return err
	}

	token := os.Getenv("BNKR_TOKEN")
	if token == "" {
		err := fmt.Errorf("BNKR_TOKEN environment variable is not set")
		t.logger.Error(err.Error())
		return err
	}

	t.logger.Info("fetching bunker clip", zap.String("clipID", clipID), zap.String("clipURL", clipURL.String()))

	clip, err := bunker.Fetch(clipID, token)
	if err != nil {
		t.logger.Error("failed to fetch bunker clip", zap.Error(err))
		return err
	}

	postURL := utils.RemoveQueryParams(clipURL.String())
	_, err = t.bot.SendVideo(&telego.SendVideoParams{
		ChatID:            telego.ChatID{ID: update.Message.Chat.ID},
		Video:             telego.InputFile{URL: clip.Link},
		Duration:          int(clip.DurationSeconds),
		Caption:           formatBunkerCaption(clip),
		ParseMode:         "HTML",
		SupportsStreaming: true,
		ReplyToMessageID:  update.Message.MessageID,
		ReplyMarkup:       openBunkerKeyboard(postURL),
	})
	if err != nil {
		t.logger.Error("failed to send bunker clip", zap.Error(err))
		return err
	}

	return nil
}
