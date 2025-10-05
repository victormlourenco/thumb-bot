package webhook

import (
	"thumb-bot/service"

	"github.com/gofiber/fiber/v2"
	"github.com/mymmrac/telego"
	"go.uber.org/zap"
)

type WebhookHandler struct {
	logger  *zap.Logger
	bot     *telego.Bot
	service *service.TelegramChannelImpl
}

func NewWebhookHandler(logger *zap.Logger, bot *telego.Bot, service *service.TelegramChannelImpl) *WebhookHandler {
	return &WebhookHandler{
		logger:  logger,
		bot:     bot,
		service: service,
	}
}

func (h *WebhookHandler) HandleWebhook(c *fiber.Ctx) error {
	// Parse the update from the request body
	var update telego.Update
	if err := c.BodyParser(&update); err != nil {
		h.logger.Error("failed to parse webhook body", zap.Error(err))
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Handle the update based on its type
	if update.Message != nil {
		message := update.Message
		h.logger.Info("received message",
			zap.String("from", message.From.Username),
			zap.String("text", message.Text))

		// Handle text messages
		if message.Text != "" {
			h.logger.Info("processing text message", zap.String("text", message.Text))

			// Process media from the text message using the service
			if err := h.service.ProcessMedia(update); err != nil {
				h.logger.Error("failed to process media", zap.Error(err))
			}
		}
	}

	return c.SendStatus(200)
}
