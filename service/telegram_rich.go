package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/mymmrac/telego"
)

type richMessageMedia struct {
	ID    string         `json:"id"`
	Media map[string]any `json:"media"`
}

type sendRichMessageRequest struct {
	ChatID          int64                        `json:"chat_id"`
	RichMessage     richMessagePayload           `json:"rich_message"`
	ReplyParameters *replyParameters             `json:"reply_parameters,omitempty"`
	ReplyMarkup     *telego.InlineKeyboardMarkup `json:"reply_markup,omitempty"`
}

type richMessagePayload struct {
	HTML                string             `json:"html"`
	Media               []richMessageMedia `json:"media,omitempty"`
	SkipEntityDetection bool               `json:"skip_entity_detection,omitempty"`
}

type replyParameters struct {
	MessageID int `json:"message_id"`
}

type telegramAPIResponse struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	Result      json.RawMessage `json:"result"`
}

func openLinkKeyboard(text, linkURL string) *telego.InlineKeyboardMarkup {
	return &telego.InlineKeyboardMarkup{
		InlineKeyboard: [][]telego.InlineKeyboardButton{{
			{Text: text, URL: linkURL},
		}},
	}
}

func openTwitterKeyboard(tweetURL string) *telego.InlineKeyboardMarkup {
	return openLinkKeyboard("Abrir no X / Twitter", tweetURL)
}

func openInstagramKeyboard(postURL string) *telego.InlineKeyboardMarkup {
	return openLinkKeyboard("Abrir no Instagram", postURL)
}

func openYouTubeKeyboard(videoURL string) *telego.InlineKeyboardMarkup {
	return openLinkKeyboard("Abrir no YouTube", videoURL)
}

func (t *TelegramChannelImpl) sendRichMessage(chatID int64, replyToMessageID int, html string, media []richMessageMedia, replyMarkup *telego.InlineKeyboardMarkup) error {
	reqBody := sendRichMessageRequest{
		ChatID: chatID,
		RichMessage: richMessagePayload{
			HTML:                html,
			Media:               media,
			SkipEntityDetection: true,
		},
		ReplyParameters: &replyParameters{MessageID: replyToMessageID},
		ReplyMarkup:     replyMarkup,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal sendRichMessage: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendRichMessage", t.bot.Token())
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("sendRichMessage request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read sendRichMessage response: %w", err)
	}

	var apiResp telegramAPIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return fmt.Errorf("unmarshal sendRichMessage response: %w", err)
	}
	if !apiResp.OK {
		return fmt.Errorf("sendRichMessage failed: %s", apiResp.Description)
	}
	return nil
}
