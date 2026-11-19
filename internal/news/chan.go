package news

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// miniChannels — публичные Telegram-каналы (t.me/s/…) как источники мини-постов.
var miniChannels = []struct{ Name, Handle string }{
	{"Код Дурова", "kod_ru"},
}

var (
	reChanText  = regexp.MustCompile(`(?s)^([^"]+)".*?<div class="tgme_widget_message_text js-message_text"[^>]*>(.*?)</div>.*?<time[^>]*datetime="([^"]+)"`)
	reTag       = regexp.MustCompile(`<[^>]+>`)
	reLeadEmoji = regexp.MustCompile(`^(?:<b>)?<tg-emoji emoji-id="(\d+)">.*?<b>([^<]+)</b>`)
	reRefLink   = regexp.MustCompile(`href="(https?://kod\.ru/[^"]+)"`)
	reBR        = regexp.MustCompile(`<br\s*/?>`)
	reURLLine   = regexp.MustCompile(`^\S*\s*https?://\S+$`)
)

// fetchChannel разбирает t.me/s/<канал>; ссылка записи — на сам пост.
func fetchChannel(ctx context.Context, name, handle string) ([]Article, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://t.me/s/"+handle, nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("t.me/s/%s: %s", handle, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var out []Article
	for _, chunk := range strings.Split(string(b), `data-post="`)[1:] {
		m := reChanText.FindStringSubmatch(chunk)
		if m == nil {
			continue
		}
		t, err := time.Parse(time.RFC3339, m[3])
		if err != nil {
			continue
		}
		title, sum, _ := strings.Cut(tidyPost(m[2]), "\n\n")
		title, sum = cleanMiniText(title, sum)
		if title == "" {
			continue
		}
		a := Article{Source: name, Title: title, Link: "https://t.me/" + m[1], Summary: sum, PublishedAt: t}
		if e := reLeadEmoji.FindStringSubmatch(strings.TrimSpace(m[2])); e != nil { // свой анимированный эмодзи поста
			a.EmojiID, a.EmojiChar = e[1], e[2]
		}
		if r := reRefLink.FindStringSubmatch(m[2]); r != nil {
			a.Ref = refKey(r[1])
		}
		out = append(out, a)
	}
	return out, nil
}

// tidyPost: HTML поста → текст абзацами без хвоста: сноски «* …», строки со ссылкой
// и подводки к ней («…не стоит:»).
func tidyPost(h string) string {
	h = reBR.ReplaceAllString(h, "\n")
	h = html.UnescapeString(reTag.ReplaceAllString(h, ""))
	var paras []string
	for _, p := range strings.Split(h, "\n\n") {
		if p = strings.TrimSpace(p); p != "" {
			paras = append(paras, p)
		}
	}
	for len(paras) > 1 {
		last := paras[len(paras)-1]
		switch {
		case strings.HasPrefix(last, "*"), reURLLine.MatchString(last):
		case strings.HasSuffix(last, ":"):
		default:
			return strings.Join(paras, "\n\n")
		}
		paras = paras[:len(paras)-1]
	}
	return strings.Join(paras, "\n\n")
}

func refKey(u string) string { return strings.TrimRight(strings.SplitN(u, "?", 2)[0], "/") }

// dropRefDupes убирает RSS-статьи, на которые уже ссылается пост канала: приходит пост (со своим эмодзи и фото).
func dropRefDupes(in []Article) []Article {
	refs := map[string]*Article{}
	for _, a := range in {
		if a.Ref != "" {
			refs[a.Ref] = nil
		}
	}
	for i, a := range in {
		if _, ok := refs[refKey(a.Link)]; ok && a.Ref == "" {
			refs[refKey(a.Link)] = &in[i]
		}
	}
	if len(refs) == 0 {
		return in
	}
	out := make([]Article, 0, len(in))
	for _, a := range in {
		if _, ok := refs[refKey(a.Link)]; ok && a.Ref == "" {
			continue
		}
		if a.Ref != "" {
			a.Alt = refs[a.Ref] // фото поста приоритетнее, статья сайта — запас
		}
		out = append(out, a)
	}
	return out
}
