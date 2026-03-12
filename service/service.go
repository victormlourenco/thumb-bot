package service

import (
	"os"
	"strconv"
	"strings"

	"github.com/mymmrac/telego"
	"go.uber.org/zap"
)

func NewTelegramService(logger *zap.Logger, bot *telego.Bot) *TelegramChannelImpl {
	tc := &TelegramChannelImpl{
		logger: logger,
		bot:    bot,
	}

	tc.initBlacklistFromEnv()

	return tc
}

type TelegramChannelImpl struct {
	logger            *zap.Logger
	bot               *telego.Bot
	blacklistedUserID map[int64]struct{}
}

func (t *TelegramChannelImpl) initBlacklistFromEnv() {
	raw := os.Getenv("TELEGRAM_USER_BLACKLIST")
	if raw == "" {
		return
	}

	ids := make(map[int64]struct{})
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if id, err := strconv.ParseInt(part, 10, 64); err == nil {
			ids[id] = struct{}{}
		} else {
			t.logger.Warn("invalid user id in TELEGRAM_USER_BLACKLIST", zap.String("value", part), zap.Error(err))
		}
	}

	if len(ids) > 0 {
		t.blacklistedUserID = ids
		t.logger.Info("telegram user blacklist initialized", zap.Int("count", len(ids)))
	}
}

func (t *TelegramChannelImpl) isUserBlacklisted(update telego.Update) bool {
	if t.blacklistedUserID == nil {
		return false
	}

	if update.Message == nil || update.Message.From == nil {
		return false
	}

	_, exists := t.blacklistedUserID[update.Message.From.ID]
	return exists
}

func (t *TelegramChannelImpl) ProcessMedia(update telego.Update) error {
	if t.isUserBlacklisted(update) {
		t.logger.Info("ignoring update from blacklisted user", zap.Int64("user_id", update.Message.From.ID))
		return nil
	}

	twitterErr := t.processTwitterMedia(update)
	if twitterErr != nil {
		t.logger.Error(twitterErr.Error())
		return twitterErr
	}
	instagramErr := t.processInstagramMedia(update)
	if instagramErr != nil {
		t.logger.Error(instagramErr.Error())
		return instagramErr
	}
	youtubeErr := t.processYouTubeMedia(update)
	if youtubeErr != nil {
		t.logger.Error(youtubeErr.Error())
		return youtubeErr
	}
	return nil
}
