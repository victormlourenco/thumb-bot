package service

import (
	"net/url"
	"thumb-bot/integration/vocaroo"
	"thumb-bot/utils"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
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

			audioMessage := tu.Audio(
				tu.ID(update.Message.Chat.ID), tu.FileFromURL(vocUrl.String()),
			).WithReplyToMessageID(update.Message.MessageID)
			_, err = t.bot.SendAudio(audioMessage)

			if err != nil {
				t.logger.Error("failed to send audio", zap.Error(err))
				return err
			}
		}
	}
	return nil
}
