package telegram

import (
	"fmt"
	"log"

	"treesheild-newsbot/assets"
	"treesheild-newsbot/internal/config"
	"treesheild-newsbot/internal/digest"
)

// PublishPreview шлёт превью дайджеста. photo — обложка дайджеста; nil — стандартная.
func PublishPreview(cfg config.Config, newsBody string, photo []byte) error {
	if len(photo) == 0 {
		photo = assets.NewsPhoto
	}
	tc, err := newTelegramController(cfg)
	if err != nil {
		return err
	}
	log.Printf("Telegram: @%s", tc.bot.Self.UserName)

	// Фото с дайджестом в подписи (одним сообщением, при необходимости текст ужимается).
	if cfg.PhotoEnabled {
		if caption, ok := digest.AssembleDigestForCaption(newsBody); ok {
			tc.setPublishButton(caption, photo)
			if err := tc.sendPhotoCaption(tc.chatID, photo, caption); err != nil {
				return err
			}
			log.Printf("Превью отправлено (фото с дайджестом)")
			return nil
		}
		// Не влезло даже после сжатия — фото отдельно, полный текст следом.
		log.Printf("Дайджест не помещается в подпись, шлю фото + текст раздельно")
		if err := tc.sendPhotoCaption(tc.chatID, photo, ""); err != nil {
			return err
		}
	}

	full := digest.AssembleDigest(newsBody)
	if n := len([]rune(full)); n > digest.TelegramMaxMessage {
		return fmt.Errorf("дайджест слишком длинный (%d симв., лимит %d)", n, digest.TelegramMaxMessage)
	}
	var pub []byte
	if cfg.PhotoEnabled {
		pub = photo
	}
	tc.setPublishButton(full, pub)
	if err := tc.sendHTML(tc.chatID, full, false); err != nil {
		return err
	}
	log.Printf("Превью отправлено")
	return nil
}
