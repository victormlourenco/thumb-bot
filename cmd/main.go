package main

import (
	"thumb-bot/internal/infra/config"
	"thumb-bot/internal/infra/logs"
	"thumb-bot/internal/listener"
	"thumb-bot/internal/service"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	tb "gopkg.in/telebot.v3"
)

func main() {
	logger := logs.NewLogger(zapcore.InfoLevel)

	config := config.Config{}
	err := config.Load()
	if err != nil {
		logger.Panic("failed to read config", zap.Error(err))
	}

	bot, err := tb.NewBot(tb.Settings{
		Token:  config.TelegramToken,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		logger.Panic("failed to create bot", zap.Error(err))
	}

	telegramService := service.NewTelegramService(logger)
	listener := listener.NewTelegramListener(logger, bot, telegramService.ProcessMedia)
	listener.Initialize()

	bot.Start()
}
