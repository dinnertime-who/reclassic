package parse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var (
	pageMarkerTitle = regexp.MustCompile(`(?i)^\{[ivxlcdm0-9]+\}$`)
	pageNumInTitle  = regexp.MustCompile(`(?i)\{[ivxlcdm0-9]+\}`)
)

// Postprocess는 전략 선택 뒤에 적용한다 (ADR-013, ADR-016).
// 원본 결과 점수는 그대로 두고, 반환된 결과만 고친다.
func Postprocess(doc *goquery.Document, res *Result) *Result {
	if res == nil {
		return nil
	}
	out := cloneResult(res)
	recoverLeadingParagraphs(doc, out)
	cleanChapterTitles(doc, out)
	restoreImageInitials(out)
	dropEmptyChapters(out)
	reindex(out)
	assignOccurrenceIDs(out)
	return out
}

func cloneResult(res *Result) *Result {
	out := *res
	out.Warnings = append([]string(nil), res.Warnings...)
	out.Chapters = make([]Chapter, len(res.Chapters))
	for i, ch := range res.Chapters {
		out.Chapters[i] = ch
		out.Chapters[i].Paragraphs = append([]Paragraph(nil), ch.Paragraphs...)
	}
	return &out
}

func recoverLeadingParagraphs(doc *goquery.Document, res *Result) {
	if doc == nil {
		return
	}
	var docParas []Paragraph
	doc.Find("p").Each(func(_ int, s *goquery.Selection) {
		if isExcluded(s) {
			return
		}
		p, ok := paragraphFrom(s)
		if !ok {
			return
		}
		docParas = append(docParas, p)
	})

	type cursor struct {
		ch, pi int
	}
	next := cursor{}
	advance := func() {
		for next.ch < len(res.Chapters) && next.pi >= len(res.Chapters[next.ch].Paragraphs) {
			next.ch++
			next.pi = 0
		}
	}
	advance()

	matched := false
	var front []Paragraph
	rebuilt := make([][]Paragraph, len(res.Chapters))

	for _, dp := range docParas {
		advance()
		if next.ch < len(res.Chapters) &&
			res.Chapters[next.ch].Paragraphs[next.pi].Text == dp.Text {
			rebuilt[next.ch] = append(rebuilt[next.ch], res.Chapters[next.ch].Paragraphs[next.pi])
			next.pi++
			matched = true
			continue
		}
		if !matched || next.ch >= len(res.Chapters) {
			if !matched {
				front = append(front, dp)
			} else if len(res.Chapters) > 0 {
				last := len(res.Chapters) - 1
				rebuilt[last] = append(rebuilt[last], dp)
			}
			continue
		}
		// 챕터 시작 경계로 쓰여 본문이 빠진 문단 (셰익스피어 THE PROLOGUE).
		rebuilt[next.ch] = append(rebuilt[next.ch], dp)
	}
	advance()
	for next.ch < len(res.Chapters) {
		rebuilt[next.ch] = append(rebuilt[next.ch], res.Chapters[next.ch].Paragraphs[next.pi:]...)
		next.ch++
		next.pi = 0
		advance()
	}

	for i := range res.Chapters {
		res.Chapters[i].Paragraphs = rebuilt[i]
	}
	if len(front) == 0 {
		return
	}
	res.Warnings = append(res.Warnings, fmt.Sprintf("첫 챕터 앞 문단 %d개를 front-matter로 보존", len(front)))
	res.Chapters = append([]Chapter{{
		Title:      "",
		Paragraphs: front,
	}}, res.Chapters...)
}

func cleanChapterTitles(doc *goquery.Document, res *Result) {
	for i := range res.Chapters {
		ch := &res.Chapters[i]
		switch ch.TitleSource {
		case TitleFromHeading:
			// 제목이 헤딩에서 왔으면 캡션·페이지 번호를 빼고 다시 읽는다 (1342).
			if ch.Anchor != "" && doc != nil {
				if t, ok := titleFromAnchor(doc, ch.Anchor); ok {
					ch.Title = t
				}
			}
		case TitleFromTOC:
			// 목차 링크 텍스트를 헤딩으로 갈아치우지 않는다 (1524, 46).
			ch.Title = stripTitleNoise(ch.Title)
		}
		if pageMarkerTitle.MatchString(strings.TrimSpace(ch.Title)) {
			ch.Title = ""
		}
	}
}

func stripTitleNoise(title string) string {
	title = pageNumInTitle.ReplaceAllString(title, " ")
	return Normalize(title)
}

func titleFromAnchor(doc *goquery.Document, id string) (string, bool) {
	sel := doc.Find("#" + cssEscapeID(id))
	if sel.Length() == 0 {
		sel = doc.Find("a[name='" + id + "']")
	}
	if sel.Length() == 0 {
		return "", false
	}
	heading := sel.Closest("h1, h2, h3")
	if heading.Length() == 0 {
		heading = sel.Find("h1, h2, h3").First()
	}
	if heading.Length() == 0 {
		name := nodeName(sel)
		if name == "h1" || name == "h2" || name == "h3" {
			heading = sel
		}
	}
	if heading.Length() == 0 {
		return "", false
	}
	return cleanedHeadingText(heading), true
}

func cleanedHeadingText(s *goquery.Selection) string {
	clone := s.Clone()
	clone.Find(".caption, .pagenum, img").Remove()
	return Normalize(clone.Text())
}

func restoreImageInitials(res *Result) {
	for i := range res.Chapters {
		for j := range res.Chapters[i].Paragraphs {
			p := &res.Chapters[i].Paragraphs[j]
			text, warn := restoreInitialFromHTML(p.HTML, p.Text)
			p.Text = text
			p.StableID = StableID(text)
			if warn != "" {
				res.Warnings = append(res.Warnings, warn)
			}
		}
	}
}

func restoreInitialFromHTML(htmlFrag, text string) (string, string) {
	if htmlFrag == "" {
		return text, ""
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlFrag))
	if err != nil {
		return text, ""
	}
	img := doc.Find(".letra img, .dropcap img").First()
	if img.Length() == 0 {
		return text, ""
	}
	alt := strings.TrimSpace(img.AttrOr("alt", ""))
	runes := []rune(alt)
	if len(runes) == 0 {
		return text, "이미지 이니셜 alt가 비어 있음"
	}
	if len(runes) != 1 {
		return text, "이미지 이니셜 alt가 한 글자가 아님: " + alt
	}
	initial := string(runes[0])
	if strings.HasPrefix(text, initial) {
		return text, ""
	}
	return Normalize(initial + text), ""
}

func dropEmptyChapters(res *Result) {
	n := 0
	kept := res.Chapters[:0]
	for _, ch := range res.Chapters {
		if len(ch.Paragraphs) == 0 {
			n++
			continue
		}
		kept = append(kept, ch)
	}
	res.Chapters = kept
	if n > 0 {
		res.Warnings = append(res.Warnings, fmt.Sprintf("문단 0개 챕터 %d개 제거", n))
	}
}

func assignOccurrenceIDs(res *Result) {
	seen := make(map[string]int)
	for i := range res.Chapters {
		for j := range res.Chapters[i].Paragraphs {
			t := res.Chapters[i].Paragraphs[j].Text
			seen[t]++
			res.Chapters[i].Paragraphs[j].StableID = StableIDN(t, seen[t])
		}
	}
}

// StableIDN은 같은 본문의 n번째 등장 id다. n==1은 현행과 같다 (ADR-016).
func StableIDN(normalized string, n int) string {
	if n <= 1 {
		return StableID(normalized)
	}
	return StableID(normalized + "#" + strconv.Itoa(n))
}
