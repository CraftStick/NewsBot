package digest

import (
	"fmt"
	"html"
	"log"
	"regexp"
	"strings"
	"unicode"

	"treesheild-newsbot/internal/news"
)

// minLinkScore — минимум два общих слова: «любую свободную» статью не подставляем,
// без уверенного совпадения пункт остаётся без ссылки и дайджест не уходит.
const minLinkScore = 24

var (
	hrefInBlock       = regexp.MustCompile(`(?i)<a\s+href="([^"]+)"`)
	extractLinkRE     = regexp.MustCompile(`(?is)<a\s+href="[^"]+">(.*?)</a>`)
	newsTitleInBoldRE = regexp.MustCompile(`(?is)<b>\s*\d{1,2}\.\s*(.*?)</b>`)
)

func extractNewsURL(block string) string {
	m := hrefInBlock.FindStringSubmatch(block)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func ExtractNewsTitle(block string) string {
	if m := extractLinkRE.FindStringSubmatch(block); len(m) >= 2 {
		return strings.TrimSpace(news.StripHTML(m[1]))
	}
	m := newsTitleInBoldRE.FindStringSubmatch(block)
	if len(m) < 2 {
		return ""
	}
	title := news.StripHTML(m[1])
	title = hrefInBlock.ReplaceAllString(title, "")
	return strings.TrimSpace(title)
}

func normalizeTitle(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if r == ' ' {
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func titleMatchScore(a, b string) int {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1000
	}
	if strings.Contains(a, b) || strings.Contains(b, a) {
		shorter := len([]rune(a))
		if l := len([]rune(b)); l < shorter {
			shorter = l
		}
		return shorter * 3
	}
	aw := strings.Fields(a)
	bw := strings.Fields(b)
	if len(aw) == 0 || len(bw) == 0 {
		return 0
	}
	common, sigA := 0, 0
	for _, w := range aw {
		if len([]rune(w)) < 3 {
			continue
		}
		sigA++
		for _, w2 := range bw {
			if sameStem(w, w2) {
				common++
				break
			}
		}
	}
	sigB := 0
	for _, w := range bw {
		if len([]rune(w)) >= 3 {
			sigB++
		}
	}
	// Общие слова должны покрывать хотя бы половину более короткой строки,
	// иначе совпали случайные «работы/сервисов» из разных историй.
	if common*2 < min(sigA, sigB) {
		return 0
	}
	return common * 12
}

// sameStem: слова равны либо различаются только окончанием («дзен» / «дзена»).
func sameStem(a, b string) bool {
	if a == b {
		return true
	}
	ra, rb := []rune(a), []rune(b)
	n := min(len(ra), len(rb))
	if n < 4 {
		return false
	}
	p := 0
	for p < n && ra[p] == rb[p] {
		p++
	}
	return p >= n-1 && p >= 4
}

// findBestArticle ищет статью по заголовку и тексту; used — уже занятые URL.
func findBestArticle(title, bodyText string, articles []news.Article, used map[string]bool) (*news.Article, int) {
	query := normalizeTitle(title)
	if query == "" {
		query = normalizeTitle(news.StripHTML(bodyText))
	}
	if query == "" {
		return nil, 0
	}

	var best *news.Article
	bestScore := 0
	for i := range articles {
		a := &articles[i]
		if a.Link == "" || used[a.Link] {
			continue
		}
		score := titleMatchScore(query, normalizeTitle(a.Title))
		if a.Summary != "" {
			if s := titleMatchScore(query, normalizeTitle(a.Summary)); s > score {
				score = s
			}
		}
		// Заголовок Gemini короткий и перефразирован — сверяем ещё и по тексту пункта.
		if full := normalizeTitle(news.StripHTML(title + " " + bodyText)); full != query {
			if s := titleMatchScore(full, normalizeTitle(a.Title)); s > score {
				score = s
			}
		}
		if score > bestScore {
			bestScore = score
			best = a
		}
	}
	return best, bestScore
}

func EnsureNewsLinks(body string, articles []news.Article) string {
	blocks := splitNewsBlocks(body)
	if len(blocks) == 0 {
		return body
	}
	if len(blocks) > RequiredNewsItems {
		blocks = blocks[:RequiredNewsItems]
	}

	used := make(map[string]bool)
	out := make([]string, 0, len(blocks))

	for _, block := range blocks {
		num := 1
		if m := newsItemHeading.FindStringSubmatch(block); len(m) >= 2 {
			num = parseNewsNum(m[1])
		}

		title := ExtractNewsTitle(block)
		text := newsBlockBody(block)
		url := extractNewsURL(block)

		var matched *news.Article
		if url == "" {
			var score int
			matched, score = findBestArticle(title, text, articles, used)
			if matched != nil && score >= minLinkScore {
				url = matched.Link
			}
		}

		if matched == nil && url != "" {
			for j := range articles {
				if articles[j].Link == url {
					matched = &articles[j]
					break
				}
			}
		}

		if title == "" && matched != nil {
			title = trimHeadline(matched.Title)
		}

		if url != "" {
			used[url] = true
			displayTitle := trimHeadline(title)
			block = fmt.Sprintf(
				"<b>%d. <a href=\"%s\">%s</a></b>\n%s",
				num,
				html.EscapeString(url),
				html.EscapeString(displayTitle),
				text,
			)
		} else {
			log.Printf("Пункт %d: не удалось найти URL (заголовок: %q)", num, title)
		}
		out = append(out, block)
	}
	return strings.Join(out, "\n\n")
}

func parseNewsNum(s string) int {
	var n int
	_, _ = fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	if n < 1 {
		return 1
	}
	return n
}

func ValidateNewsLinks(body string) error {
	blocks := splitNewsBlocks(body)
	if len(blocks) < RequiredNewsItems {
		return fmt.Errorf("мало блоков для проверки ссылок")
	}
	for i := 0; i < RequiredNewsItems; i++ {
		if extractNewsURL(blocks[i]) == "" {
			return fmt.Errorf("пункт %d без ссылки на источник", i+1)
		}
	}
	return nil
}
