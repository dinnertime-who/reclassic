package parse

import "github.com/PuerkitoBio/goquery"

type SectionChapter struct{}

func (SectionChapter) Name() string { return "section-chapter" }

func (SectionChapter) Extract(doc *goquery.Document) (*Result, error) {
	res := &Result{Strategy: "section-chapter"}
	doc.Find("section.chapter, div.chapter").Each(func(_ int, s *goquery.Selection) {
		if isExcluded(s) {
			return
		}
		title := ""
		if h := s.Find("h1, h2, h3").First(); h.Length() > 0 {
			title = headingText(h)
		}
		anchor, _ := s.Attr("id")
		ch := Chapter{
			Title:      title,
			Anchor:     anchor,
			Paragraphs: collectParagraphs(s),
		}
		if len(ch.Paragraphs) == 0 && title == "" {
			return
		}
		res.Chapters = append(res.Chapters, ch)
	})
	if len(res.Chapters) == 0 {
		res.Warnings = append(res.Warnings, "section.chapter / div.chapter 없음")
	}
	reindex(res)
	return res, nil
}
