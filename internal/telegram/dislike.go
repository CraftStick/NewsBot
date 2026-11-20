package telegram

import (
	"encoding/json"
	"os"
	"regexp"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	cbDislike    = "dis:"
	dislikedFile = "disliked.jsonl" // рядом с pending/, в рабочем каталоге бота
)

var hrefRE = regexp.MustCompile(`href="([^"]+)"`)

// pendingRow — кнопки под ждущим постом: «Пропустить» просто убирает из очереди,
// «Удалить» ещё и записывает пост как не понравившийся.
func pendingRow(key string) []tgbotapi.InlineKeyboardButton {
	return tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🗑 Пропустить", cbSkip+key),
		tgbotapi.NewInlineKeyboardButtonData("👎 Удалить", cbDislike+key))
}

// noteDisliked дописывает заголовок и ссылки поста в disliked.jsonl; до skipPending, пока файл жив.
func noteDisliked(key string) {
	if !pendingKeyRE.MatchString(key) {
		return
	}
	for _, p := range pendingList() {
		if p.key != key {
			continue
		}
		var links []string
		for _, m := range hrefRE.FindAllStringSubmatch(p.html, -1) {
			links = append(links, m[1])
		}
		rec, _ := json.Marshal(map[string]any{"time": time.Now().Format(time.RFC3339), "title": p.title, "links": links})
		if f, err := os.OpenFile(dislikedFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
			_, _ = f.Write(append(rec, '\n'))
			f.Close()
		}
	}
}

// dislikedCount — сколько постов вы удалили как не понравившиеся.
func dislikedCount() int {
	b, _ := os.ReadFile(dislikedFile)
	n := 0
	for _, c := range b {
		if c == '\n' {
			n++
		}
	}
	return n
}
