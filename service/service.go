package service

import (
	"github.com/mymmrac/telego"
	"go.uber.org/zap"
)

func NewTelegramService(logger *zap.Logger, bot *telego.Bot) *TelegramChannelImpl {
	return &TelegramChannelImpl{
		logger: logger,
		bot:    bot,
	}
}

type TelegramChannelImpl struct {
	logger *zap.Logger
	bot    *telego.Bot
}

func (t *TelegramChannelImpl) ProcessMedia(update telego.Update) error {
	twitterErr := t.processTwitterMedia(update)
	if twitterErr != nil {
		t.logger.Error(twitterErr.Error())
		return twitterErr
	}
	instagramErr := t.processInstagramMedia(update)
	if instagramErr != nil {
		t.logger.Error(instagramErr.Error())
		return instagramErr
	}
	youtubeErr := t.processYouTubeMedia(update)
	if youtubeErr != nil {
		t.logger.Error(youtubeErr.Error())
		return youtubeErr
	}
	return nil
}
