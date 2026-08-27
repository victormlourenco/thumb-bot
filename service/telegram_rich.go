package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/mymmrac/telego"
)

const (
	telegramCaptionLimit = 1024
	telegramMessageLimit = 4096
	previewMaxRunes      = 280
	previewMaxLines      = 4
	collapseMinRestRunes = 80
	readMoreSummary      = "Read more..."
)

func exceedsTelegramLimit(text string, hasMedia bool) bool {
	limit := telegramMessageLimit
	if hasMedia {
		limit = telegramCaptionLimit
	}
	return utf8.RuneCountInString(text) > limit
}

func needsReadMore(text string) bool {
	_, rest := splitReadMore(text)
	return rest != ""
}

func splitReadMore(text string) (preview, rest string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)
	if text == "" {
		return "", ""
	}

	runes := []rune(text)
	cut := -1
	lines := 1
	for i, r := range runes {
		if r == '\n' {
			lines++
			if lines > previewMaxLines {
				cut = i
				break
			}
		}
		if i+1 >= previewMaxRunes && cut < 0 {
			cut = breakAtWord(runes, i+1)
			break
		}
	}

	if cut < 0 || cut >= len(runes) {
		return text, ""
	}

	preview = strings.TrimRight(string(runes[:cut]), " \t\n")
	rest = strings.TrimLeft(string(runes[cut:]), " \t\n")
	if rest == "" || utf8.RuneCountInString(rest) < collapseMinRestRunes {
		return text, ""
	}
	return preview, rest
}

func breakAtWord(runes []rune, at int) int {
	if at >= len(runes) {
		return -1
	}
	for i := at; i > 0 && at-i < 40; i-- {
		switch runes[i-1] {
		case ' ', '\t', '\n':
			return i - 1
		}
	}
	return at
}

func richPlainHTML(text string) string {
	return strings.ReplaceAll(escapeText(text), "\n", "<br/>")
}

func writeReadMoreDetails(sb *strings.Builder, rest string) {
	sb.WriteString("<details><summary>")
	sb.WriteString(readMoreSummary)
	sb.WriteString("</summary>\n<p>")
	sb.WriteString(richPlainHTML(rest))
	sb.WriteString("</p>\n</details>\n")
}

func writeHeaderAndCollapsibleBody(sb *strings.Builder, headerHTML, body string) {
	preview, rest := splitReadMore(body)

	sb.WriteString("<p>")
	sb.WriteString(headerHTML)
	if preview != "" {
		sb.WriteString("<br/>")
		sb.WriteString(richPlainHTML(preview))
	}
	sb.WriteString("</p>\n")

	if rest != "" {
		writeReadMoreDetails(sb, rest)
	}
}

func formatCaptionWithReadMore(header, body string) string {
	if body == "" {
		return header
	}
	preview, rest := splitReadMore(body)
	var caption strings.Builder
	caption.WriteString(header)
	caption.WriteString(":\n")
	caption.WriteString(escapeText(preview))
	if rest != "" {
		caption.WriteString("\n<blockquote expandable>")
		caption.WriteString(escapeText(rest))
		caption.WriteString("</blockquote>")
	}
	return caption.String()
}

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

func openFacebookKeyboard(postURL string) *telego.InlineKeyboardMarkup {
	return openLinkKeyboard("Abrir no Facebook", postURL)
}

func openMarketplaceKeyboard(listingURL string) *telego.InlineKeyboardMarkup {
	return openLinkKeyboard("Abrir no Marketplace", listingURL)
}

func openYouTubeKeyboard(videoURL string) *telego.InlineKeyboardMarkup {
	return openLinkKeyboard("Abrir no YouTube", videoURL)
}

func openBunkerKeyboard(clipURL string) *telego.InlineKeyboardMarkup {
	return openLinkKeyboard("Abrir no Bunker", clipURL)
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
