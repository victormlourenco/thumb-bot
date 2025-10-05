package handler

import (
	"net/http"
	"os"
	"thumb-bot/infra/logs"
	"thumb-bot/service"
	"thumb-bot/webhook"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/mymmrac/telego"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	app *fiber.App
	bot *telego.Bot
)

func handler() http.HandlerFunc {
	// Initialize logger
	logger := logs.NewLogger(zapcore.InfoLevel)

	// Get bot token from environment variable
	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		logger.Fatal("TELEGRAM_TOKEN environment variable is not set")
	}

	// Initialize bot with token
	var err error
	bot, err = telego.NewBot(token)
	if err != nil {
		logger.Fatal("failed to create bot", zap.Error(err))
	}

	// Create service
	telegramService := service.NewTelegramService(logger, bot)

	// Create webhook handler
	webhookHandler := webhook.NewWebhookHandler(logger, bot, telegramService)

	// Create Fiber app
	app = fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// Add middleware
	app.Use(recover.New())

	// Health check endpoint
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
		})
	})

	// Webhook endpoint
	app.Post("/webhook", webhookHandler.HandleWebhook)

	return adaptor.FiberApp(app)
}

// Handler is the main function that Vercel will call
func Handler(w http.ResponseWriter, r *http.Request) {
	r.RequestURI = r.URL.String()
	handler().ServeHTTP(w, r)
}
