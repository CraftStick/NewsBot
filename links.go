package main

import (
	"fmt"
	"html"
	"log"
	"regexp"
	"strings"
	"unicode"
)

var (
	hrefInBlock   = regexp.MustCompile(`(?i)<a\s+href="([^"]+)"`)
	extractLinkRE = regexp.MustCompile(`(?is)<a\s+href="[^"]+">(.*?)</a>`)
)

func extractNewsURL(block string) string {
	m := hrefInBlock.FindStringSubmatch(block)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func extractNewsTitle(block string) string {
	if m := extractLinkRE.FindStringSubmatch(block); len(m) >= 2 {
		return strings.TrimSpace(stripHTML(m[1]))
	}
	re := regexp.MustCompile(`(?is)<b>\s*\d{1,2}\.\s*(.*?)</b>`)
	m := re.FindStringSubmatch(block)
	if len(m) < 2 {
		return ""
	}
	title := stripHTML(m[1])
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
	common := 0
	for _, w := range aw {
		if len([]rune(w)) < 3 {
			continue
		}
		for _, w2 := range bw {
			if w == w2 {
				common++
				break
			}
		}
	}
	return common * 12
}

// findBestArticle ищет статью по заголовку и тексту; used — уже занятые URL.
func findBestArticle(title, bodyText string, articles []Article, used map[string]bool) (*Article, int) {
	query := normalizeTitle(title)
	if query == "" {
		query = normalizeTitle(stripHTML(bodyText))
	}
	if query == "" {
		return nil, 0
	}

	var best *Article
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
		if score > bestScore {
			bestScore = score
			best = a
		}
	}
	return best, bestScore
}

func pickUnusedArticle(articles []Article, used map[string]bool, preferForeign bool) *Article {
	var best *Article
	for i := range articles {
		a := &articles[i]
		if a.Link == "" || used[a.Link] {
			continue
		}
		isForeign := ruNewsPriority(a.Title, a.Summary) == 0
		if preferForeign && !isForeign {
			continue
		}
		if !preferForeign && isForeign && best != nil {
			continue
		}
		if best == nil || a.RUPriority > best.RUPriority {
			best = a
		}
	}
	if best != nil {
		return best
	}
	for i := range articles {
		a := &articles[i]
		if a.Link != "" && !used[a.Link] {
			return a
		}
	}
	return nil
}

func ensureNewsLinks(body string, articles []Article) string {
	blocks := splitNewsBlocks(body)
	if len(blocks) == 0 {
		return body
	}

	used := make(map[string]bool)
	out := make([]string, 0, len(blocks))

	for i, block := range blocks {
		num := i + 1
		if m := newsItemHeading.FindStringSubmatch(block); len(m) >= 2 {
			num = parseNewsNum(m[1])
		}

		title := extractNewsTitle(block)
		text := newsBlockBody(block)
		url := extractNewsURL(block)

		var matched *Article
		if url == "" {
			var score int
			matched, score = findBestArticle(title, text, articles, used)
			if matched != nil && score >= 12 {
				url = matched.Link
			}
		}

		preferForeign := num == foreignNewsItemNum
		if url == "" {
			matched = pickUnusedArticle(articles, used, preferForeign)
			if matched != nil {
				url = matched.Link
				log.Printf("Пункт %d: ссылка подобрана по ленте (%s)", num, matched.Source)
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

func validateNewsLinks(body string) error {
	blocks := splitNewsBlocks(body)
	if len(blocks) < requiredNewsItems {
		return fmt.Errorf("мало блоков для проверки ссылок")
	}
	for i := 0; i < requiredNewsItems; i++ {
		if extractNewsURL(blocks[i]) == "" {
			return fmt.Errorf("пункт %d без ссылки на источник", i+1)
		}
	}
	return nil
}
