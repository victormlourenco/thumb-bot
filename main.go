package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"thumb-bot/infra/logs"
	"thumb-bot/service"
	"thumb-bot/webhook"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/mymmrac/telego"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// Initialize logger
	logger := logs.NewLogger(zapcore.InfoLevel)
	defer logger.Sync()

	// Get bot token from environment variable
	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		logger.Fatal("TELEGRAM_TOKEN environment variable is not set")
	}

	// Initialize bot with token
	bot, err := telego.NewBot(token)
	if err != nil {
		logger.Fatal("failed to create bot", zap.Error(err))
	}

	// Create service
	telegramService := service.NewTelegramService(logger, bot)

	// Create webhook handler
	webhookHandler := webhook.NewWebhookHandler(logger, bot, telegramService)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		DisableStartupMessage: false,
	})

	// Add middleware
	app.Use(recover.New())
	app.Use(cors.New())

	// Health check endpoint
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "ok",
			"timestamp": time.Now().Unix(),
		})
	})

	// Webhook endpoint
	app.Post("/webhook", webhookHandler.HandleWebhook)

	// Polling mode for local development
	logger.Info("Starting bot in polling mode for local development")
	startPollingMode(bot, telegramService, logger)

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

func startPollingMode(bot *telego.Bot, service *service.TelegramChannelImpl, logger *zap.Logger) {
	// Delete webhook to ensure polling works
	if err := bot.DeleteWebhook(&telego.DeleteWebhookParams{DropPendingUpdates: true}); err != nil {
		logger.Warn("Failed to delete webhook", zap.Error(err))
	}

	// Create update channel
	updates, err := bot.UpdatesViaLongPolling(&telego.GetUpdatesParams{
		AllowedUpdates: []string{"message"},
		Timeout:        60,
	})
	if err != nil {
		logger.Fatal("Failed to start polling", zap.Error(err))
	}

	// Process updates
	go func() {
		for update := range updates {
			logger.Info("Received update",
				zap.Int("update_id", update.UpdateID),
				zap.String("type", getUpdateType(update)))

			// Process the update
			if err := service.ProcessMedia(update); err != nil {
				logger.Error("Failed to process update", zap.Error(err))
			}
		}
	}()

	logger.Info("Polling started successfully")
}

func getUpdateType(update telego.Update) string {
	if update.Message != nil {
		return "message"
	}
	if update.CallbackQuery != nil {
		return "callback_query"
	}
	if update.InlineQuery != nil {
		return "inline_query"
	}
	return "unknown"
}
