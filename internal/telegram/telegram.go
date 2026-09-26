package telegram

import (
	"fmt"
	"log"

	"treesheild-newsbot/assets"
	"treesheild-newsbot/internal/config"
	"treesheild-newsbot/internal/digest"
)

func PublishPreview(cfg config.Config, newsBody string) error {
	tc, err := newTelegramController(cfg)
	if err != nil {
		return err
	}
	log.Printf("Telegram: @%s", tc.bot.Self.UserName)

	const hint = "<i>Превью готово.</i> Скопируйте <b>следующее</b> сообщение в канал. " +
		"Не подошло — кнопка ниже или /digest."
	if err := tc.sendHTML(tc.chatID, hint, true); err != nil {
		return err
	}

	// Фото с дайджестом в подписи (одним сообщением, при необходимости текст ужимается).
	if cfg.PhotoEnabled {
		if caption, ok := digest.AssembleDigestForCaption(newsBody); ok {
			if err := tc.sendPhotoCaption(tc.chatID, assets.NewsPhoto, caption); err != nil {
				return err
			}
			log.Printf("Превью отправлено (подсказка + фото с дайджестом)")
			return nil
		}
		// Не влезло даже после сжатия — фото отдельно, полный текст следом.
		log.Printf("Дайджест не помещается в подпись, шлю фото + текст раздельно")
		if err := tc.sendPhotoCaption(tc.chatID, assets.NewsPhoto, ""); err != nil {
			return err
		}
	}

	full := digest.AssembleDigest(newsBody)
	if n := len([]rune(full)); n > digest.TelegramMaxMessage {
		return fmt.Errorf("дайджест слишком длинный (%d симв., лимит %d)", n, digest.TelegramMaxMessage)
	}
	if err := tc.sendHTML(tc.chatID, full, false); err != nil {
		return err
	}
	log.Printf("Превью отправлено")
	return nil
}
