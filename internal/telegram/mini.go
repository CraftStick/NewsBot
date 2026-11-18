package telegram

import (
	"encoding/json"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"treesheild-newsbot/internal/config"
)

// SendMini шлёт мини-пост в личку: фото с подписью (photo == nil — просто текст),
// с кнопкой «Опубликовать», если она настроена. Предпросмотр ссылки отключён.
func SendMini(cfg config.Config, html string, photo []byte) error {
	tc, err := newTelegramController(cfg)
	if err != nil {
		return err
	}
	tc.setPublishButton(html, photo)
	var kb *tgbotapi.InlineKeyboardMarkup
	if tc.publishKB != nil {
		k := tgbotapi.NewInlineKeyboardMarkup(tc.publishKB...)
		kb = &k
	}
	if len(photo) > 0 {
		msg := tgbotapi.NewPhoto(tc.chatID, tgbotapi.FileBytes{Name: "news.jpg", Bytes: photo})
		msg.ParseMode, msg.Caption = tgbotapi.ModeHTML, html
		if kb != nil {
			msg.ReplyMarkup = *kb
		}
		_, err = tc.bot.Send(msg)
		return err
	}
	msg := tgbotapi.NewMessage(tc.chatID, html)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true
	if kb != nil {
		msg.ReplyMarkup = *kb
	}
	_, err = tc.bot.Send(msg)
	return err
}

// SendText шлёт HTML-сообщение в личку без кнопок и предпросмотра ссылок.
func SendText(cfg config.Config, html string) error {
	tc, err := newTelegramController(cfg)
	if err != nil {
		return err
	}
	msg := tgbotapi.NewMessage(tc.chatID, html)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true
	_, err = tc.bot.Send(msg)
	return err
}

// SetMenuApp ставит кнопку меню бота, открывающую мини-апку.
func SetMenuApp(cfg config.Config, url string) error {
	tc, err := newTelegramController(cfg)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(map[string]any{"type": "web_app", "text": "Новости", "web_app": map[string]string{"url": url}})
	_, err = tc.bot.MakeRequest("setChatMenuButton", tgbotapi.Params{"menu_button": string(b)})
	return err
}
