package service

import (
	"go.uber.org/zap"
	tb "gopkg.in/telebot.v3"
)

func NewTelegramService(logger *zap.Logger) *TelegramChannelImpl {
	return &TelegramChannelImpl{
		logger: logger,
	}
}

type TelegramChannelImpl struct {
	logger *zap.Logger
}

func (t *TelegramChannelImpl) ProcessMedia(c tb.Context) error {
	twitterErr := t.processTwitterMedia(c)
	if twitterErr != nil {
		t.logger.Error(twitterErr.Error())
		return twitterErr
	}
	instagramErr := t.processInstagramMedia(c)
	if instagramErr != nil {
		t.logger.Error(instagramErr.Error())
		return instagramErr
	}
	vocarooErr := t.processVocarooMedia(c)
	if vocarooErr != nil {
		t.logger.Error(vocarooErr.Error())
		return vocarooErr
	}
	return nil
}
