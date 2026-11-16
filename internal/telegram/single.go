package telegram

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"treesheild-newsbot/internal/config"
)

var (
	headingNumRE = regexp.MustCompile(`^<b>\s*\d{1,2}\.\s*`)
	pendingKeyRE = regexp.MustCompile(`^[0-9a-f]{16}$`)
)

// PublishSinglePreview шлёт одиночный пост ТОЛЬКО в личку на проверку. В канал
// он уходит лишь по кнопке «Опубликовать» (юзербот, PUBLISH_CMD) или копированием вручную.
func PublishSinglePreview(cfg config.Config, post, image string) error {
	tc, err := newTelegramController(cfg)
	if err != nil {
		return err
	}
	if err := tc.sendHTML(tc.chatID, "<i>Пост на сегодня (превью).</i> Подходит — скопируйте <b>следующее</b> сообщение в канал; нет — просто пропустите.", false); err != nil {
		return err
	}
	html := formatSinglePost(post)
	if cfg.PublishCmd == "" {
		return tc.sendHTML(tc.chatID, html, false)
	}
	_, id := PickEmoji(html)
	key, err := savePending(html, id, image)
	if err != nil {
		return err
	}
	msg := tgbotapi.NewMessage(tc.chatID, html)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.DisableWebPagePreview = true
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📣 Опубликовать в канал", cbPublish+key)))
	_, err = tc.bot.Send(msg)
	return err
}

// setPublishButton сохраняет дайджест и включает кнопку «Опубликовать» на следующем
// сообщении. Без PUBLISH_CMD или при ошибке записи кнопки нет (публикуют вручную).
func (tc *telegramController) setPublishButton(html string, photo []byte) {
	if tc.cfg.PublishCmd == "" {
		return
	}
	image := ""
	if len(photo) > 0 {
		sum := sha1.Sum([]byte(html))
		path, err := filepath.Abs(filepath.Join(pendingDir, hex.EncodeToString(sum[:8])+".jpg"))
		if err == nil && os.MkdirAll(pendingDir, 0o700) == nil &&
			os.WriteFile(path, photo, 0o600) == nil {
			image = path
		}
	}
	key, err := savePending(html, "", image)
	if err != nil {
		log.Printf("кнопка публикации не создана: %v", err)
		return
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("📣 Опубликовать в канал", cbPublish+key)))
	tc.publishKB = &kb
}

const (
	cbPublish  = "pub:"
	pendingDir = "pending"
)

type pendingPost struct {
	HTML    string `json:"html"`
	EmojiID string `json:"emoji_id"`
	Image   string `json:"image,omitempty"`
}

// savePending кладёт пост на диск под коротким ключом: callback_data ≤ 64 байт,
// а кнопку может нажать другой процесс, чем отправивший превью.
func savePending(html, emojiID, image string) (string, error) {
	sum := sha1.Sum([]byte(html))
	key := hex.EncodeToString(sum[:8])
	b, _ := json.Marshal(pendingPost{html, emojiID, image})
	if err := os.MkdirAll(pendingDir, 0o700); err != nil {
		return "", err
	}
	sweepPending()
	return key, os.WriteFile(filepath.Join(pendingDir, key+".json"), b, 0o600)
}

// sweepPending удаляет заготовки старше недели: неопубликованные превью иначе копятся вечно.
func sweepPending() {
	files, _ := filepath.Glob(filepath.Join(pendingDir, "*"))
	for _, f := range files {
		if st, err := os.Stat(f); err == nil && time.Since(st.ModTime()) > 7*24*time.Hour {
			_ = os.Remove(f)
		}
	}
}

var publishMu sync.Mutex

// publishPending отдаёт сохранённый пост юзерботу. Файл удаляется только после успеха:
// повторное нажатие после публикации ничего не отправит.
func publishPending(cfg config.Config, key string) error {
	publishMu.Lock() // два быстрых нажатия иначе опубликуют пост дважды
	defer publishMu.Unlock()
	if !pendingKeyRE.MatchString(key) {
		return errors.New("неверный ключ")
	}
	path := filepath.Join(pendingDir, key+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		return errors.New("пост уже опубликован или устарел")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", cfg.PublishCmd)
	cmd.Stdin = bytes.NewReader(b)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("юзербот: %v: %s", err, strings.TrimSpace(string(out)))
	}
	var p pendingPost
	if json.Unmarshal(b, &p) == nil && strings.HasPrefix(p.Image, "/") {
		_ = os.Remove(p.Image) // локальная шапка дайджеста
	}
	return os.Remove(path)
}

// topicEmoji — эмодзи по теме. ID — анимированный custom emoji для юзербота
// (бот их отправить не может, в превью идёт обычный символ).
var topicEmoji = []struct {
	emoji, id string
	words     []string
}{
	{"✅", "5278411813468269386", []string{"разблокир", "сняли блокир", "снял блокир", "оправдал"}},
	{"🚫", "5767151002666929821", []string{"блокир", "заблокир", "роскомнадзор", "ркн", "запрет", "ограничен"}},
	{"🔓", "5278602437001767574", []string{"утечк", "слив", "персональн"}},
	{"🛸", "5350513460985743273", []string{"vpn", "впн", "шифрован", "приватн"}},
	{"🛡️", "5348230282140993341", []string{"хакер", "взлом", "уязвим", "ddos", "атак", "вирус", "malware", "фишинг", "мошенн"}},
	{"🤖", "5785346296608988394", []string{"нейросет", "искусственн", "chatgpt", "openai", "gpt", "llm"}},
	{"📱", "5348482508390419372", []string{"telegram", "телеграм", "whatsapp", "мессенджер"}},
	{"☕️", "5348414471813483004", []string{"суд", "штраф", "закон", "госдум"}},
	{"🔍", "5803403369913520877", []string{"оператор", "мтс", "билайн", "мегафон", "теле2", "мобильн", "связ"}},
	{"🔎", "5291742567321328330", []string{"яндекс", "vk", "вк ", "сбер", "госуслуг", "ozon", "wildberries"}},
	{"🌐", "5222258908529207849", []string{"сша", "китай", "евросоюз", "европ", "apple", "google", "microsoft", "meta", "amazon", "nvidia", "tesla"}},
}

// defaultEmoji — когда тема не распознана.
const (
	defaultEmoji   = "📰"
	defaultEmojiID = "5951891646445523477"
)

// PickEmoji возвращает символ и id анимированного эмодзи по теме текста.
func PickEmoji(text string) (emoji, id string) {
	text = strings.ToLower(text)
	for _, t := range topicEmoji {
		for _, w := range t.words {
			if strings.Contains(text, w) {
				return t.emoji, t.id
			}
		}
	}
	return defaultEmoji, defaultEmojiID
}

// formatSinglePost: «эмодзи заголовок», пустая строка, текст.
func formatSinglePost(post string) string {
	out := headingNumRE.ReplaceAllString(strings.TrimSpace(post), "<b>")
	out = strings.Replace(out, "</b>\n", "</b>\n\n", 1)
	e, _ := PickEmoji(out)
	return e + " " + out
}
