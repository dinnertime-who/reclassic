package parse

import "github.com/PuerkitoBio/goquery"

type SingleChapter struct{}

func (SingleChapter) Name() string { return "single-chapter" }

func (SingleChapter) Extract(doc *goquery.Document) (*Result, error) {
	title := ""
	if h := doc.Find("h1").First(); h.Length() > 0 && !isExcluded(h) {
		title = headingText(h)
	}
	res := &Result{
		Strategy: "single-chapter",
		Chapters: []Chapter{{
			Title:      title,
			Paragraphs: collectParagraphs(doc.Selection),
		}},
	}
	if len(res.Chapters[0].Paragraphs) == 0 {
		res.Warnings = append(res.Warnings, "문단을 찾지 못함")
	}
	reindex(res)
	return res, nil
}
