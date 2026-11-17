package telegram

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"treesheild-newsbot/assets"

	"treesheild-newsbot/internal/config"
)

func TestPickEmojiID(t *testing.T) {
	if _, id := PickEmoji("Новый VPN под блокировкой"); id != "5767151002666929821" { // блокировка раньше VPN
		t.Fatalf("id=%s", id)
	}
	if _, id := PickEmoji("Вышел смартфон"); id != defaultEmojiID {
		t.Fatalf("id=%s", id)
	}
}

func TestPickEmojiNewTopics(t *testing.T) {
	cases := map[string]string{
		"Утечка данных клиентов банка":    "5278602437001767574",
		"МТС расширила сеть":              "5803403369913520877",
		"Яндекс запустил сервис":          "5291742567321328330",
		"Google открыла офис в США":       "5222258908529207849",
		"Роскомнадзор разблокировал сайт": "5278411813468269386",
		"OpenAI выпустила модель":         "5785346296608988394",
	}
	for in, want := range cases {
		if _, id := PickEmoji(in); id != want {
			t.Errorf("%q: id=%s, ждали %s", in, id, want)
		}
	}
}

func TestPublishPending(t *testing.T) {
	t.Chdir(t.TempDir())
	key, err := savePending("<b>x</b>", "123", "")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{PublishCmd: "cat > got.json"}
	if err := publishPending(cfg, key); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile("got.json"); !strings.Contains(string(b), `"emoji_id":"123"`) {
		t.Fatalf("stdin: %s", b)
	}
	if publishPending(cfg, key) == nil { // повтор не должен публиковать
		t.Fatal("повторная публикация прошла")
	}
	if publishPending(cfg, "../x") == nil {
		t.Fatal("ключ не проверен")
	}
}

func TestSetPublishButtonDigest(t *testing.T) {
	t.Chdir(t.TempDir())
	tc := &telegramController{cfg: config.Config{PublishCmd: "cat > got.json"}}
	tc.setPublishButton("<b>d</b>", assets.NewsPhoto)
	if tc.publishKB == nil {
		t.Fatal("кнопки нет")
	}
	key := strings.TrimPrefix(*tc.publishKB[0][0].CallbackData, cbPublish)
	if err := publishPending(tc.cfg, key); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile("got.json"); !strings.Contains(string(b), `"image":"/`) {
		t.Fatalf("нет пути к шапке: %s", b)
	}
	if m, _ := filepath.Glob("pending/*"); len(m) != 0 {
		t.Fatalf("остались файлы: %v", m)
	}
}

func TestSweepPending(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := savePending("old", "", ""); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-8 * 24 * time.Hour)
	files, _ := filepath.Glob(filepath.Join(pendingDir, "*"))
	for _, f := range files {
		_ = os.Chtimes(f, old, old)
	}
	if _, err := savePending("new", "", ""); err != nil {
		t.Fatal(err)
	}
	if files, _ := filepath.Glob(filepath.Join(pendingDir, "*")); len(files) != 1 {
		t.Fatalf("ждали 1 свежую заготовку, есть %v", files)
	}
}

func TestPreviewKeyboard(t *testing.T) {
	tc := &telegramController{cfg: config.Config{PublishCmd: "cat"}}
	if n := len(tc.previewKeyboard().InlineKeyboard); n != 3 {
		t.Fatalf("без публикации три ряда, а %d", n)
	}
	t.Chdir(t.TempDir())
	tc.setPublishButton("<b>d</b>", nil)
	if n := len(tc.previewKeyboard().InlineKeyboard); n != 4 {
		t.Fatalf("ожидалось 4 ряда, а %d", n)
	}
}
