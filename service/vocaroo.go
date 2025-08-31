package service

import (
	"net/url"
	"thumb-bot/integration/vocaroo"
	"thumb-bot/utils"

	"github.com/mymmrac/telego"
	"go.uber.org/zap"
)

var vocarooHosts = []string{
	"vocaroo.com",
	"voca.ro",
	"www.vocaroo.com",
}

func (t *TelegramChannelImpl) processVocarooMedia(update telego.Update) error {
	if update.Message == nil || update.Message.Text == "" {
		return nil
	}

	payload := update.Message.Text
	links := utils.ExtractLinks(payload)
	if len(links) == 0 {
		return nil
	}

	vocUrl, err := url.Parse(links[0])
	if err != nil {
		t.logger.Error("failed to parse vocUrl", zap.Error(err))
		return err
	}

	for _, host := range vocarooHosts {
		if vocUrl.Host == host {
			t.logger.Info("fetching audio", zap.String("vocUrl", vocUrl.String()))
			response, _, err := vocaroo.Fetch(vocUrl.String())
			if err != nil {
				t.logger.Error("failed to fetch audio", zap.Error(err))
				return err
			}

			if response == nil {
				t.logger.Error("failed to fetch audio")
				return err
			}

			// Send audio using the bot instance
			_, err = t.bot.SendAudio(&telego.SendAudioParams{
				ChatID:           telego.ChatID{ID: update.Message.Chat.ID},
				Audio:            telego.InputFile{URL: vocUrl.String()},
				ReplyToMessageID: update.Message.MessageID,
			})
			if err != nil {
				t.logger.Error("failed to send audio", zap.Error(err))
				return err
			}
		}
	}
	return nil
}
