package telegram

import (
	"regexp"

	"treesheild-newsbot/internal/config"
)

var headingNumRE = regexp.MustCompile(`^<b>\s*\d{1,2}\.\s*`)

// PublishSinglePreview шлёт одиночный пост ТОЛЬКО в личку на проверку: в канал
// бот сам ничего не отправляет, пост копируется вручную.
func PublishSinglePreview(cfg config.Config, post string) error {
	tc, err := newTelegramController(cfg)
	if err != nil {
		return err
	}
	if err := tc.sendHTML(tc.chatID, "<i>Пост на сегодня (превью).</i> Подходит — скопируйте <b>следующее</b> сообщение в канал; нет — просто пропустите.", false); err != nil {
		return err
	}
	return tc.sendHTML(tc.chatID, headingNumRE.ReplaceAllString(post, "<b>"), false)
}
