package parse

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type AnchorTOC struct{}

func (AnchorTOC) Name() string { return "anchor-toc" }

type tocEntry struct {
	id    string
	title string
}

func (AnchorTOC) Extract(doc *goquery.Document) (*Result, error) {
	res := &Result{Strategy: "anchor-toc"}
	entries := collectTOC(doc)
	if len(entries) < 2 {
		res.Warnings = append(res.Warnings, "목차 앵커가 충분하지 않음")
		return res, nil
	}

	targets := make(map[string]string, len(entries))
	order := make([]string, 0, len(entries))
	for _, e := range entries {
		if _, ok := targets[e.id]; ok {
			continue
		}
		targets[e.id] = e.title
		order = append(order, e.id)
	}

	cur := -1
	doc.Find("p, [id], a[name], h1, h2, h3").Each(func(_ int, s *goquery.Selection) {
		if isExcluded(s) {
			return
		}
		id := elementAnchor(s)
		if id != "" {
			if title, ok := targets[id]; ok {
				if heading := nodeName(s); heading == "h1" || heading == "h2" || heading == "h3" {
					if ht := headingText(s); ht != "" {
						title = ht
					}
				}
				res.Chapters = append(res.Chapters, Chapter{Title: title, Anchor: id})
				cur = len(res.Chapters) - 1
				return
			}
		}
		if nodeName(s) != "p" || cur < 0 {
			return
		}
		p, ok := paragraphFrom(s)
		if !ok {
			return
		}
		p.Idx = len(res.Chapters[cur].Paragraphs)
		res.Chapters[cur].Paragraphs = append(res.Chapters[cur].Paragraphs, p)
	})

	if len(res.Chapters) == 0 {
		res.Warnings = append(res.Warnings, "목차 타겟을 본문에서 찾지 못함: "+strings.Join(order, ", "))
	}
	reindex(res)
	return res, nil
}

func collectTOC(doc *goquery.Document) []tocEntry {
	seen := map[string]bool{}
	var out []tocEntry

	add := func(s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok {
			return
		}
		id := hrefID(href)
		if id == "" || seen[id] {
			return
		}
		if doc.Find("#"+cssEscapeID(id)+", a[name='"+id+"']").Length() == 0 {
			return
		}
		seen[id] = true
		out = append(out, tocEntry{id: id, title: Normalize(s.Text())})
	}

	doc.Find(".toc a[href^='#'], nav a[href^='#'], .contents a[href^='#'], #toc a[href^='#'], #contents a[href^='#'], [class*='toc'] a[href^='#']").Each(func(_ int, s *goquery.Selection) {
		add(s)
	})
	if len(out) >= 3 {
		return out
	}

	doc.Find("a[href^='#']").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		id := hrefID(href)
		if !looksLikeChapterAnchor(id, Normalize(s.Text())) {
			return
		}
		add(s)
	})
	return out
}

func hrefID(href string) string {
	href = strings.TrimSpace(href)
	if i := strings.Index(href, "#"); i >= 0 {
		href = href[i+1:]
	}
	href, _ = url.QueryUnescape(href)
	return strings.TrimSpace(href)
}

func elementAnchor(s *goquery.Selection) string {
	if id, ok := s.Attr("id"); ok && id != "" {
		return id
	}
	if name, ok := s.Attr("name"); ok && name != "" {
		return name
	}
	if id, ok := s.Find("[id]").First().Attr("id"); ok {
		return id
	}
	if name, ok := s.Find("a[name]").First().Attr("name"); ok {
		return name
	}
	return ""
}

func looksLikeChapterAnchor(id, text string) bool {
	hay := strings.ToLower(id + " " + text)
	keys := []string{
		"chap", "chapter", "book", "stave", "letter", "part",
		"canto", "act", "scene", "story", "tale", "epist",
	}
	for _, k := range keys {
		if strings.Contains(hay, k) {
			return true
		}
	}
	return false
}

// cssEscapeID는 goquery/sizzle이 특수문자가 있는 id를 찾도록 이스케이프한다.
func cssEscapeID(id string) string {
	var b strings.Builder
	for _, r := range id {
		if r == ':' || r == '.' || r == '[' || r == ']' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
