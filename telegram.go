package main

import (
	"fmt"
	"log"
)

func publishPreview(cfg Config, htmlText string) error {
	if n := len([]rune(htmlText)); n > telegramMaxMessage {
		return fmt.Errorf("дайджест слишком длинный (%d симв., лимит %d)", n, telegramMaxMessage)
	}

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
	if err := tc.sendHTML(tc.chatID, htmlText, false); err != nil {
		return err
	}
	log.Printf("Превью отправлено (2 сообщения + кнопка обновить)")
	return nil
}
