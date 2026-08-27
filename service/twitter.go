package service

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"thumb-bot/integration/fxtwitter"
	"thumb-bot/integration/vxtwitter"
	"thumb-bot/utils"
	"unicode/utf8"

	"github.com/mymmrac/telego"
	"go.uber.org/zap"
)

const maxQuotedTextRunes = 248

var twitterHosts = []string{
	"twitter.com",
	"mobile.twitter.com",
	"www.twitter.com",
	"t.co",
	"x.com",
	"www.x.com",
}

func (t *TelegramChannelImpl) processTwitterMedia(update telego.Update) error {
	if update.Message == nil || update.Message.Text == "" {
		return nil
	}

	payload := update.Message.Text
	links := utils.ExtractLinks(payload)
	if len(links) == 0 {
		return nil
	}

	if strings.Contains(links[0], "t.co") {
		links[0], _ = expandShortURL(links[0])
	}

	twUrl, err := url.Parse(links[0])
	if err != nil {
		t.logger.Error("failed to parse twUrl", zap.Error(err))
		return err
	}

	for _, host := range twitterHosts {
		if twUrl.Host == host {
			t.logger.Info("fetching tweet", zap.String("twUrl", twUrl.String()))

			// Try fxtwitter first
			fxResponse, fxErr := fxtwitter.Fetch(twUrl.Path)
			if fxErr == nil && fxResponse.Code == 200 {
				t.logger.Info("using fxtwitter provider")
				return t.processFxtwitterResponse(update, fxResponse)
			}

			// Fallback to vxtwitter
			t.logger.Info("fxtwitter failed, trying vxtwitter", zap.Error(fxErr))
			vxResponse, vxErr := vxtwitter.Fetch(twUrl.Path)
			if vxErr != nil {
				t.logger.Error("both fxtwitter and vxtwitter failed", zap.Error(vxErr))
				return vxErr
			}

			t.logger.Info("using vxtwitter provider")
			return t.processVxtwitterResponse(update, vxResponse)
		}
	}
	return nil
}

func expandShortURL(shortURL string) (string, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Head(shortURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return resp.Header.Get("Location"), nil
}

func escapeText(text string) string {
	return html.EscapeString(html.UnescapeString(text))
}

func truncateQuotedText(text string) string {
	if utf8.RuneCountInString(text) <= maxQuotedTextRunes {
		return text
	}
	runes := []rune(text)
	return string(runes[:maxQuotedTextRunes]) + "\n..."
}

func formatAuthorHeader(name, screenName string) string {
	return fmt.Sprintf(
		`<b><a href="https://x.com/%s">%s</a> (<code>@%s</code>)</b>`,
		screenName,
		escapeText(name),
		escapeText(screenName),
	)
}

func writeFxTweetHeaderAndText(sb *strings.Builder, tweet fxtwitter.Tweet) {
	writeHeaderAndCollapsibleBody(sb, formatAuthorHeader(tweet.Author.Name, tweet.Author.ScreenName), tweet.Text)
}

func formatFxTweetCaption(tweet fxtwitter.Tweet) string {
	caption := formatCaptionWithReadMore(
		formatAuthorHeader(tweet.Author.Name, tweet.Author.ScreenName),
		tweet.Text,
	)

	if tweet.Quote != nil {
		quotedText := truncateQuotedText(tweet.Quote.Text)
		caption += fmt.Sprintf("\n<blockquote><i>Quoting</i> %s:\n%s</blockquote>",
			formatAuthorHeader(tweet.Quote.Author.Name, tweet.Quote.Author.ScreenName),
			escapeText(quotedText),
		)
	}

	return caption
}

func formatVxTweetCaption(response vxtwitter.Response) string {
	return formatCaptionWithReadMore(
		formatAuthorHeader(response.UserName, response.UserScreenName),
		response.Text,
	)
}

type resolvedMedia struct {
	URL  string
	Type string // photo | video
}

func resolveFxMediaItems(items []fxtwitter.MediaItem) []resolvedMedia {
	var resolved []resolvedMedia
	for _, media := range items {
		bestURL, mediaType, found := fxtwitter.GetBestMediaForTelegram(media)
		if !found {
			continue
		}
		resolved = append(resolved, resolvedMedia{
			URL:  utils.RemoveQueryParams(bestURL),
			Type: mediaType,
		})
	}
	return resolved
}

func appendRichMedia(sb *strings.Builder, medias []resolvedMedia, startID int) []richMessageMedia {
	mediaList := make([]richMessageMedia, 0, len(medias))
	if len(medias) == 0 {
		return mediaList
	}

	// Gallery tweets (2+ items) use <tg-collage> per Bot API rich HTML examples.
	useCollage := len(medias) > 1
	if useCollage {
		sb.WriteString("<tg-collage>")
	}

	for i, media := range medias {
		id := strconv.Itoa(startID + i)
		switch media.Type {
		case "video":
			sb.WriteString(fmt.Sprintf(`<video src="tg://video?id=%s"></video>`, id))
			mediaList = append(mediaList, richMessageMedia{
				ID: id,
				Media: map[string]any{
					"type":  "video",
					"media": media.URL,
				},
			})
		default:
			sb.WriteString(fmt.Sprintf(`<img src="tg://photo?id=%s"/>`, id))
			mediaList = append(mediaList, richMessageMedia{
				ID: id,
				Media: map[string]any{
					"type":  "photo",
					"media": media.URL,
				},
			})
		}
	}

	if useCollage {
		sb.WriteString("</tg-collage>\n")
	} else {
		sb.WriteString("\n")
	}
	return mediaList
}

func buildFxQuoteArticle(tweet fxtwitter.Tweet, mainMedias, quoteMedias []resolvedMedia) (string, []richMessageMedia) {
	var htmlBuilder strings.Builder
	var mediaList []richMessageMedia
	quoteMediaPromoted := len(mainMedias) == 0 && len(quoteMedias) > 0

	writeFxTweetHeaderAndText(&htmlBuilder, tweet)

	if !quoteMediaPromoted && len(mainMedias) > 0 {
		mediaList = append(mediaList, appendRichMedia(&htmlBuilder, mainMedias, 1)...)
	}

	// Block quotation per Bot API: <blockquote>…<cite>Author</cite></blockquote>
	htmlBuilder.WriteString("<blockquote>\n")
	writeFxTweetHeaderAndText(&htmlBuilder, *tweet.Quote)

	if quoteMediaPromoted {
		mediaList = append(mediaList, appendRichMedia(&htmlBuilder, quoteMedias, 1)...)
	} else if len(quoteMedias) > 0 {
		mediaList = append(mediaList, appendRichMedia(&htmlBuilder, quoteMedias, len(mediaList)+1)...)
	}

	fmt.Fprintf(&htmlBuilder, "<cite>%s</cite>\n", escapeText(tweet.Quote.Author.Name))
	htmlBuilder.WriteString("</blockquote>\n")
	return htmlBuilder.String(), mediaList
}

func buildFxMediaArticle(tweet fxtwitter.Tweet, medias []resolvedMedia) (string, []richMessageMedia) {
	var htmlBuilder strings.Builder
	writeFxTweetHeaderAndText(&htmlBuilder, tweet)
	mediaList := appendRichMedia(&htmlBuilder, medias, 1)
	return htmlBuilder.String(), mediaList
}

func buildVxMediaArticle(response vxtwitter.Response, medias []resolvedMedia) (string, []richMessageMedia) {
	var htmlBuilder strings.Builder
	writeHeaderAndCollapsibleBody(&htmlBuilder, formatAuthorHeader(response.UserName, response.UserScreenName), response.Text)
	mediaList := appendRichMedia(&htmlBuilder, medias, 1)
	return htmlBuilder.String(), mediaList
}

func buildFxMediaGroup(items []resolvedMedia, caption string) []telego.InputMedia {
	var mediaGroup []telego.InputMedia
	for i, media := range items {
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
		case "photo":
			mediaGroup = append(mediaGroup, &telego.InputMediaPhoto{
				Media:     telego.InputFile{URL: media.URL},
				Caption:   itemCaption,
				ParseMode: "HTML",
				Type:      "photo",
			})
		}
	}
	return mediaGroup
}

func (t *TelegramChannelImpl) sendTwitterMediaWithButton(update telego.Update, medias []resolvedMedia, caption, tweetURL string) error {
	keyboard := openTwitterKeyboard(tweetURL)
	chatID := telego.ChatID{ID: update.Message.Chat.ID}
	replyTo := update.Message.MessageID

	if len(medias) == 1 {
		media := medias[0]
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

	// Albums can't carry inline keyboards — send as one media group only.
	mediaGroup := buildFxMediaGroup(medias, caption)
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

func (t *TelegramChannelImpl) processFxtwitterResponse(update telego.Update, response fxtwitter.Response) error {
	tweetURL := response.Tweet.URL
	if tweetURL == "" {
		tweetURL = fmt.Sprintf("https://x.com/%s/status/%s", response.Tweet.Author.ScreenName, response.Tweet.ID)
	}
	keyboard := openTwitterKeyboard(tweetURL)

	var mainMedias []resolvedMedia
	if response.Tweet.Media != nil {
		mainMedias = resolveFxMediaItems(response.Tweet.Media.All)
	}

	var quoteMedias []resolvedMedia
	if response.Tweet.Quote != nil && response.Tweet.Quote.Media != nil {
		quoteMedias = resolveFxMediaItems(response.Tweet.Quote.Media.All)
	}

	medias := mainMedias
	if len(medias) == 0 {
		medias = quoteMedias
	}
	caption := formatFxTweetCaption(response.Tweet)

	quoteWithMedia := response.Tweet.Quote != nil && (len(mainMedias) > 0 || len(quoteMedias) > 0)
	gallery := len(medias) > 1 && response.Tweet.Quote == nil
	tooLong := response.Tweet.IsNoteTweet || exceedsTelegramLimit(caption, len(medias) > 0) || needsReadMore(response.Tweet.Text)
	if response.Tweet.Quote != nil && needsReadMore(response.Tweet.Quote.Text) {
		tooLong = true
	}

	// Quotes with media, galleries, and long/note tweets use sendRichMessage
	// (regular captions cap at 1024 chars; text messages at 4096).
	if quoteWithMedia || gallery || tooLong {
		var htmlBody string
		var richMedia []richMessageMedia
		if response.Tweet.Quote != nil {
			htmlBody, richMedia = buildFxQuoteArticle(response.Tweet, mainMedias, quoteMedias)
		} else {
			htmlBody, richMedia = buildFxMediaArticle(response.Tweet, medias)
		}
		err := t.sendRichMessage(update.Message.Chat.ID, update.Message.MessageID, htmlBody, richMedia, keyboard)
		if err == nil {
			return nil
		}
		t.logger.Warn("sendRichMessage failed, falling back", zap.Error(err))
	}

	if len(medias) > 0 {
		if err := t.sendTwitterMediaWithButton(update, medias, caption, tweetURL); err != nil {
			t.logger.Error("failed to send twitter media", zap.Error(err))
			return err
		}
		return nil
	}

	if response.Tweet.Text != "" {
		_, err := t.bot.SendMessage(&telego.SendMessageParams{
			ChatID:                telego.ChatID{ID: update.Message.Chat.ID},
			Text:                  caption,
			ParseMode:             "HTML",
			DisableWebPagePreview: true,
			ReplyToMessageID:      update.Message.MessageID,
			ReplyMarkup:           keyboard,
		})
		if err != nil {
			t.logger.Error("failed to send message", zap.Error(err))
			return err
		}
	}
	return nil
}

func (t *TelegramChannelImpl) processVxtwitterResponse(update telego.Update, response vxtwitter.Response) error {
	tweetURL := response.TweetURL
	if tweetURL == "" {
		tweetURL = fmt.Sprintf("https://x.com/%s/status/%s", response.UserScreenName, response.TweetID)
	}
	caption := formatVxTweetCaption(response)
	keyboard := openTwitterKeyboard(tweetURL)

	var medias []resolvedMedia
	for _, media := range response.MediaExtended {
		mediaType := "photo"
		if media.Type == "video" {
			mediaType = "video"
		}
		medias = append(medias, resolvedMedia{
			URL:  utils.RemoveQueryParams(media.URL),
			Type: mediaType,
		})
	}

	// Galleries and long tweets use one rich message; normal posts use regular sends.
	if len(medias) > 1 || exceedsTelegramLimit(caption, len(medias) > 0) || needsReadMore(response.Text) {
		htmlBody, richMedia := buildVxMediaArticle(response, medias)
		err := t.sendRichMessage(update.Message.Chat.ID, update.Message.MessageID, htmlBody, richMedia, keyboard)
		if err == nil {
			return nil
		}
		t.logger.Warn("sendRichMessage failed for vxtwitter, falling back", zap.Error(err))
	}

	if len(medias) > 0 {
		if err := t.sendTwitterMediaWithButton(update, medias, caption, tweetURL); err != nil {
			t.logger.Error("failed to send media group", zap.Error(err))
			return err
		}
		return nil
	}

	if response.Text != "" {
		_, err := t.bot.SendMessage(&telego.SendMessageParams{
			ChatID:                telego.ChatID{ID: update.Message.Chat.ID},
			Text:                  caption,
			ParseMode:             "HTML",
			DisableWebPagePreview: true,
			ReplyToMessageID:      update.Message.MessageID,
			ReplyMarkup:           keyboard,
		})
		if err != nil {
			t.logger.Error("failed to send message", zap.Error(err))
			return err
		}
	}
	return nil
}
