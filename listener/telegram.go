package listener

import (
	"github.com/mymmrac/telego"
	"go.uber.org/zap"
)

func NewTelegramListener(logger *zap.Logger, bot *telego.Bot, onMessage func(telego.Update) error) *TelegramChannelImpl {
	return &TelegramChannelImpl{
		logger:    logger,
		bot:       bot,
		onMessage: onMessage,
	}
}

type TelegramChannelImpl struct {
	logger    *zap.Logger
	bot       *telego.Bot
	onMessage func(telego.Update) error
}

func (t *TelegramChannelImpl) Initialize() {
	// Note: This will need to be called from the handler setup
	// The actual handler registration should be done where the handler is created
}
