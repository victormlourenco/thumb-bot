package service

import (
	"go.uber.org/zap"
	tb "gopkg.in/telebot.v3"
	"net/url"
	"thumb-bot/internal/integration/vocaroo"
	"thumb-bot/internal/utils"
)

var vocarooHosts = []string{
	"vocaroo.com",
	"voca.ro",
	"www.vocaroo.com",
}

func (t *TelegramChannelImpl) processVocarooMedia(c tb.Context) error {
	payload := c.Message().Text
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

			options := &tb.SendOptions{
				ReplyTo: c.Message(),
			}

			file := tb.FromReader(response)
			err = c.Send(&tb.Audio{File: file}, options)
			if err != nil {
				t.logger.Error("failed to send album", zap.Error(err))
				return err
			}
		}
	}
	return nil
}
