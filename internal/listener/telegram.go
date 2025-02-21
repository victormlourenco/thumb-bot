package listener

import (
	"go.uber.org/zap"
	tb "gopkg.in/telebot.v3"
)

func NewTelegramListener(logger *zap.Logger, bot *tb.Bot, onMessage tb.HandlerFunc) *TelegramChannelImpl {
	return &TelegramChannelImpl{
		logger:    logger,
		bot:       bot,
		onMessage: onMessage,
	}
}

type TelegramChannelImpl struct {
	logger    *zap.Logger
	bot       *tb.Bot
	onMessage tb.HandlerFunc
}

func (t *TelegramChannelImpl) Initialize() {
	t.bot.Handle(tb.OnText, t.onMessage)
}
