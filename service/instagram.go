package service

import (
	"fmt"
	"net/url"
	"strings"
	"thumb-bot/integration/instagram"
	"thumb-bot/utils"

	"github.com/mymmrac/telego"
	"go.uber.org/zap"
)

var instagramHosts = []string{
	"instagram.com",
	"www.instagram.com",
	"m.instagram.com",
	"instagr.am",
	"www.instagr.am",
}

func formatInstagramCaption(response instagram.InstagramResponse) string {
	username := response.PostInfo.OwnerUsername
	name := response.PostInfo.OwnerFullname
	if name == "" {
		name = username
	}

	header := fmt.Sprintf(
		`<b><a href="https://www.instagram.com/%s/">%s</a> (<code>@%s</code>)</b>`,
		username,
		escapeText(name),
		escapeText(username),
	)

	if response.PostInfo.Caption == "" {
		return header
	}
	return fmt.Sprintf("%s:\n%s", header, escapeText(response.PostInfo.Caption))
}

func writeInstagramHeaderAndText(sb *strings.Builder, response instagram.InstagramResponse) {
	username := response.PostInfo.OwnerUsername
	name := response.PostInfo.OwnerFullname
	if name == "" {
		name = username
	}

	header := fmt.Sprintf(
		`<b><a href="https://www.instagram.com/%s/">%s</a> (<code>@%s</code>)</b>`,
		username,
		escapeText(name),
		escapeText(username),
	)
	caption := escapeText(response.PostInfo.Caption)

	sb.WriteString("<p>")
	sb.WriteString(header)
	if caption != "" {
		sb.WriteString("<br/>")
		sb.WriteString(strings.ReplaceAll(caption, "\n", "<br/>"))
	}
	sb.WriteString("</p>\n")
}

func buildInstagramRichArticle(response instagram.InstagramResponse, medias []resolvedMedia) (string, []richMessageMedia) {
	var htmlBuilder strings.Builder
	writeInstagramHeaderAndText(&htmlBuilder, response)
	mediaList := appendRichMedia(&htmlBuilder, medias, 1)
	return htmlBuilder.String(), mediaList
}

func resolveInstagramMedia(details []instagram.MediaDetail) []resolvedMedia {
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

func (t *TelegramChannelImpl) sendInstagramMediaWithButton(update telego.Update, details []instagram.MediaDetail, caption, postURL string) error {
	keyboard := openInstagramKeyboard(postURL)
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
		case "image":
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

func (t *TelegramChannelImpl) processInstagramMedia(update telego.Update) error {
	if update.Message == nil || update.Message.Text == "" {
		return nil
	}

	payload := update.Message.Text
	links := utils.ExtractLinks(payload)
	if len(links) == 0 {
		return nil
	}

	instaUrl, err := url.Parse(links[0])
	if err != nil {
		t.logger.Error("failed to parse instaUrl", zap.Error(err))
		return err
	}

	for _, host := range instagramHosts {
		if instaUrl.Host != host {
			continue
		}

		if strings.Contains(instaUrl.String(), "/stories") {
			return nil
		}

		t.logger.Info("fetching instagram post", zap.String("instaUrl", instaUrl.String()))
		response, err := instagram.GetURL(instaUrl.String())
		if err != nil {
			t.logger.Error("failed to instagram post", zap.Error(err))
			return err
		}

		if len(response.MediaDetails) == 0 {
			return nil
		}

		postURL := utils.RemoveQueryParams(instaUrl.String())
		medias := resolveInstagramMedia(response.MediaDetails)
		caption := formatInstagramCaption(response)

		// Carousels use one rich message; normal single posts use regular sends.
		if len(medias) > 1 {
			keyboard := openInstagramKeyboard(postURL)
			htmlBody, richMedia := buildInstagramRichArticle(response, medias)
			if err := t.sendRichMessage(update.Message.Chat.ID, update.Message.MessageID, htmlBody, richMedia, keyboard); err == nil {
				return nil
			}
			t.logger.Warn("sendRichMessage failed for instagram album, falling back", zap.Error(err))
		}

		if err := t.sendInstagramMediaWithButton(update, response.MediaDetails, caption, postURL); err != nil {
			t.logger.Error("failed to send instagram media", zap.Error(err))
			return err
		}
		return nil
	}
	return nil
}
