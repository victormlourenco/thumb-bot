package service

import (
	"fmt"
	"net/url"
	"strings"
	"thumb-bot/integration/facebook"
	"thumb-bot/utils"

	"github.com/mymmrac/telego"
	"go.uber.org/zap"
)

var facebookHosts = []string{
	"facebook.com",
	"www.facebook.com",
	"m.facebook.com",
	"web.facebook.com",
	"mbasic.facebook.com",
	"fb.com",
	"www.fb.com",
	"fb.watch",
	"www.fb.watch",
	"l.facebook.com",
	"lm.facebook.com",
}

func formatFacebookHeader(response facebook.Response) string {
	name := response.AuthorName
	if name == "" {
		name = "Facebook"
	}

	authorURL := response.AuthorURL
	if authorURL == "" {
		authorURL = "https://www.facebook.com"
	}

	username := facebookUsername(authorURL)
	if username != "" {
		return fmt.Sprintf(
			`<b><a href="%s">%s</a> (<code>@%s</code>)</b>`,
			authorURL,
			escapeText(name),
			escapeText(username),
		)
	}

	return fmt.Sprintf(`<b><a href="%s">%s</a></b>`, authorURL, escapeText(name))
}

func formatFacebookCaption(response facebook.Response) string {
	if response.IsMarketplace {
		return formatMarketplaceCaption(response)
	}
	return formatCaptionWithReadMore(formatFacebookHeader(response), response.Caption)
}

func marketplaceListingURL(response facebook.Response) string {
	if response.PostURL != "" {
		return response.PostURL
	}
	return "https://www.facebook.com/marketplace"
}

func marketplaceTitle(response facebook.Response) string {
	if response.Title != "" {
		return response.Title
	}
	if response.AuthorName != "" {
		return response.AuthorName
	}
	return "Facebook Marketplace"
}

func marketplaceMetaHTML(response facebook.Response) string {
	var meta []string
	if response.Price != "" {
		meta = append(meta, "<b>"+escapeText(response.Price)+"</b>")
	}
	if response.Location != "" {
		meta = append(meta, escapeText(response.Location))
	}
	return strings.Join(meta, " · ")
}

func marketplaceDescription(response facebook.Response) string {
	desc := strings.TrimSpace(response.Caption)
	title := marketplaceTitle(response)
	if title != "" && strings.HasPrefix(desc, title) {
		desc = strings.TrimSpace(strings.TrimPrefix(desc, title))
	}
	return desc
}

func formatMarketplaceCaption(response facebook.Response) string {
	listingURL := marketplaceListingURL(response)
	title := marketplaceTitle(response)

	var parts []string
	parts = append(parts, fmt.Sprintf(`<b><a href="%s">%s</a></b>`, listingURL, escapeText(title)))

	if meta := marketplaceMetaHTML(response); meta != "" {
		parts = append(parts, meta)
	}

	desc := marketplaceDescription(response)
	if desc == "" {
		return strings.Join(parts, "\n")
	}

	preview, rest := splitReadMore(desc)
	parts = append(parts, escapeText(preview))
	caption := strings.Join(parts, "\n")
	if rest != "" {
		caption += "\n<blockquote expandable>" + escapeText(rest) + "</blockquote>"
	}
	return caption
}

func writeFacebookHeaderAndText(sb *strings.Builder, response facebook.Response) {
	if response.IsMarketplace {
		writeMarketplaceHeaderAndText(sb, response)
		return
	}
	writeHeaderAndCollapsibleBody(sb, formatFacebookHeader(response), response.Caption)
}

func writeMarketplaceHeaderAndText(sb *strings.Builder, response facebook.Response) {
	listingURL := marketplaceListingURL(response)
	title := marketplaceTitle(response)
	header := fmt.Sprintf(`<b><a href="%s">%s</a></b>`, listingURL, escapeText(title))
	preview, rest := splitReadMore(marketplaceDescription(response))

	sb.WriteString("<p>")
	sb.WriteString(header)
	if meta := marketplaceMetaHTML(response); meta != "" {
		sb.WriteString("<br/>")
		sb.WriteString(meta)
	}
	if preview != "" {
		sb.WriteString("<br/>")
		sb.WriteString(richPlainHTML(preview))
	}
	sb.WriteString("</p>\n")
	if rest != "" {
		writeReadMoreDetails(sb, rest)
	}
}

func buildFacebookRichArticle(response facebook.Response, medias []resolvedMedia) (string, []richMessageMedia) {
	var htmlBuilder strings.Builder
	writeFacebookHeaderAndText(&htmlBuilder, response)
	mediaList := appendRichMedia(&htmlBuilder, medias, 1)
	return htmlBuilder.String(), mediaList
}

func resolveFacebookMedia(details []facebook.MediaDetail) []resolvedMedia {
	var medias []resolvedMedia
	for _, media := range details {
		mediaType := "photo"
		if media.Type == "video" {
			mediaType = "video"
		}
		medias = append(medias, resolvedMedia{URL: media.URL, Type: mediaType})
	}
	return medias
}

func facebookUsername(authorURL string) string {
	parsed, err := url.Parse(authorURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 1 || parts[0] == "" || parts[0] == "profile.php" || parts[0] == "marketplace" {
		return ""
	}
	return parts[0]
}

func facebookKeyboard(postURL string, marketplace bool) *telego.InlineKeyboardMarkup {
	if marketplace {
		return openMarketplaceKeyboard(postURL)
	}
	return openFacebookKeyboard(postURL)
}

func (t *TelegramChannelImpl) sendFacebookMediaWithButton(update telego.Update, details []facebook.MediaDetail, caption, postURL string, marketplace bool) error {
	keyboard := facebookKeyboard(postURL, marketplace)
	chatID := telego.ChatID{ID: update.Message.Chat.ID}
	replyTo := update.Message.MessageID

	if len(details) == 1 {
		media := details[0]
		switch media.Type {
		case "video":
			_, err := t.bot.SendVideo(&telego.SendVideoParams{
				ChatID:           chatID,
				Video:            telego.InputFile{URL: media.URL},
				Caption:          caption,
				ParseMode:        "HTML",
				ReplyToMessageID: replyTo,
				ReplyMarkup:      keyboard,
			})
			return err
		default:
			_, err := t.bot.SendPhoto(&telego.SendPhotoParams{
				ChatID:           chatID,
				Photo:            telego.InputFile{URL: media.URL},
				Caption:          caption,
				ParseMode:        "HTML",
				ReplyToMessageID: replyTo,
				ReplyMarkup:      keyboard,
			})
			return err
		}
	}

	var mediaGroup []telego.InputMedia
	for i, media := range details {
		itemCaption := ""
		if i == 0 {
			itemCaption = caption
		}
		switch media.Type {
		case "video":
			mediaGroup = append(mediaGroup, &telego.InputMediaVideo{
				Media:     telego.InputFile{URL: media.URL},
				Caption:   itemCaption,
				ParseMode: "HTML",
				Type:      "video",
			})
		default:
			mediaGroup = append(mediaGroup, &telego.InputMediaPhoto{
				Media:     telego.InputFile{URL: media.URL},
				Caption:   itemCaption,
				ParseMode: "HTML",
				Type:      "photo",
			})
		}
	}

	if len(mediaGroup) == 0 {
		return nil
	}

	_, err := t.bot.SendMediaGroup(&telego.SendMediaGroupParams{
		ChatID:           chatID,
		Media:            mediaGroup,
		ReplyToMessageID: replyTo,
	})
	return err
}

func (t *TelegramChannelImpl) processFacebookMedia(update telego.Update) error {
	if update.Message == nil || update.Message.Text == "" {
		return nil
	}

	links := utils.ExtractLinks(update.Message.Text)
	if len(links) == 0 {
		return nil
	}

	rawURL := facebook.CanonicalizeURL(links[0])
	fbURL, err := url.Parse(rawURL)
	if err != nil {
		t.logger.Error("failed to parse facebook URL", zap.Error(err))
		return err
	}

	isFacebook := false
	for _, host := range facebookHosts {
		if fbURL.Host == host {
			isFacebook = true
			break
		}
	}
	if !isFacebook {
		return nil
	}

	if strings.Contains(fbURL.Path, "/stories") {
		return nil
	}

	t.logger.Info("fetching facebook post", zap.String("facebookURL", fbURL.String()))
	response, err := facebook.Fetch(fbURL.String())
	if err != nil {
		t.logger.Error("failed to fetch facebook post", zap.Error(err))
		return err
	}

	postURL := response.PostURL
	if postURL == "" {
		postURL = utils.RemoveQueryParams(fbURL.String())
	}

	medias := resolveFacebookMedia(response.MediaDetails)
	caption := formatFacebookCaption(response)
	keyboard := facebookKeyboard(postURL, response.IsMarketplace)

	if len(medias) > 1 || exceedsTelegramLimit(caption, len(medias) > 0) || needsReadMore(response.Caption) {
		htmlBody, richMedia := buildFacebookRichArticle(response, medias)
		if sendErr := t.sendRichMessage(update.Message.Chat.ID, update.Message.MessageID, htmlBody, richMedia, keyboard); sendErr == nil {
			return nil
		} else {
			t.logger.Warn("sendRichMessage failed for facebook, falling back", zap.Error(sendErr))
		}
	}

	if len(response.MediaDetails) > 0 {
		if err := t.sendFacebookMediaWithButton(update, response.MediaDetails, caption, postURL, response.IsMarketplace); err != nil {
			t.logger.Error("failed to send facebook media", zap.Error(err))
			return err
		}
		return nil
	}

	if response.Caption != "" || response.AuthorName != "" {
		_, err := t.bot.SendMessage(&telego.SendMessageParams{
			ChatID:                telego.ChatID{ID: update.Message.Chat.ID},
			Text:                  caption,
			ParseMode:             "HTML",
			DisableWebPagePreview: true,
			ReplyToMessageID:      update.Message.MessageID,
			ReplyMarkup:           keyboard,
		})
		if err != nil {
			t.logger.Error("failed to send facebook message", zap.Error(err))
			return err
		}
	}
	return nil
}
