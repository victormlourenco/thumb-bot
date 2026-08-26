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
	tc.initBunkerAllowedChatsFromEnv()

	return tc
}

type TelegramChannelImpl struct {
	logger               *zap.Logger
	bot                  *telego.Bot
	blacklistedUserID    map[int64]struct{}
	bunkerAllowedChatIDs map[int64]struct{}
}

func parseIDSet(raw, envName string, logger *zap.Logger) map[int64]struct{} {
	if raw == "" {
		return nil
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
			logger.Warn("invalid id in "+envName, zap.String("value", part), zap.Error(err))
		}
	}

	if len(ids) == 0 {
		return nil
	}
	return ids
}

func (t *TelegramChannelImpl) initBlacklistFromEnv() {
	ids := parseIDSet(os.Getenv("TELEGRAM_USER_BLACKLIST"), "TELEGRAM_USER_BLACKLIST", t.logger)
	if ids == nil {
		return
	}
	t.blacklistedUserID = ids
	t.logger.Info("telegram user blacklist initialized", zap.Int("count", len(ids)))
}

func (t *TelegramChannelImpl) initBunkerAllowedChatsFromEnv() {
	ids := parseIDSet(os.Getenv("BNKR_ALLOWED_CHAT_IDS"), "BNKR_ALLOWED_CHAT_IDS", t.logger)
	if ids == nil {
		return
	}
	t.bunkerAllowedChatIDs = ids
	t.logger.Info("bunker allowed chats initialized", zap.Int("count", len(ids)))
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
	facebookErr := t.processFacebookMedia(update)
	if facebookErr != nil {
		t.logger.Error(facebookErr.Error())
		return facebookErr
	}
	youtubeErr := t.processYouTubeMedia(update)
	if youtubeErr != nil {
		t.logger.Error(youtubeErr.Error())
		return youtubeErr
	}
	bunkerErr := t.processBunkerMedia(update)
	if bunkerErr != nil {
		t.logger.Error(bunkerErr.Error())
		return bunkerErr
	}
	return nil
}
